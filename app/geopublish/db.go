package geopublish

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/forklift"
	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/tus/tusd/pkg/handler"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"
)

type UploadsDB struct {
	handler *Handler
	db      boil.Executor
	queue   *forklift.Forklift
	logger  logging.KVLogger
}

type markFunc func(ctx context.Context, hook handler.HookEvent, user *models.User) error

func (d *UploadsDB) guardUser(ctx context.Context, mf markFunc, exec boil.Executor, hook handler.HookEvent) error {
	user, err := d.handler.getUserFromRequest(&http.Request{
		Header: hook.HTTPRequest.Header,
	})
	if err != nil {
		return err
	}
	return mf(ctx, hook, user)
}

func (d *UploadsDB) listenToHandler(upHandler *Handler) {
	d.handler = upHandler

	go func() {
		for {
			d.logger.Debug("retrieving next upload result from queue")
			ur, err := d.queue.GetUploadProcessResult()
			if err != nil {
				d.logger.Warn("failed to get upload result", "err", err)
				continue
			}
			l := logging.TracedLogger(d.logger, ur)
			ctx := logging.AddToContext(context.Background(), l)
			l.Debug("retrieved upload result")
			err = d.saveUploadProcessingResult(ctx, ur.UploadID, ur.Response, ur.Error)
			if err != nil {
				l.Warn("could not save upload result")
			} else {
				metrics.UploadsProcessed.Inc()
				l.Info("upload result processed")
			}
		}
	}()

	go func() {
		for {
			var (
				gerr  error
				eName string
				e     handler.HookEvent
			)
			l := d.logger.With("upload_id", e.Upload.ID)
			ctx := logging.AddToContext(context.Background(), l)
			select {
			case e = <-upHandler.CreatedUploads:
				metrics.UploadsCreated.Inc()
				eName = "CreatedUploads"
				err := d.guardUser(ctx, d.markUploadCreated, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling created uploads signal: %w", err)
				}
			case e = <-upHandler.UploadProgress:
				eName = "UploadProgress"
				err := d.guardUser(ctx, d.markUploadProgress, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling upload progress signal: %w", err)
				}
			case e = <-upHandler.TerminatedUploads:
				metrics.UploadsCanceled.Inc()
				eName = "TerminatedUploads"
				err := d.guardUser(ctx, d.markUploadTerminated, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling terminated upload signal: %w", err)
				}
			}
			if gerr != nil {
				metrics.UploadsDBErrors.Inc()
				l.Warn("upload signal error", "err", gerr)
			} else {
				l.Debug("handled upload signal", "event_name", eName)
			}
		}

	}()
}

func (d *UploadsDB) get(id string, userID int) (*models.Upload, error) {
	mods := []qm.QueryMod{
		models.UploadWhere.ID.EQ(id),
		qm.Load(models.UploadRels.Query),
	}
	if userID > 0 {
		mods = append(mods, models.UploadWhere.UserID.EQ(null.IntFrom(userID)))
	}
	return models.Uploads(mods...).One(d.db)
}

func (d *UploadsDB) markUploadCreated(ctx context.Context, hook handler.HookEvent, user *models.User) error {

	upload := models.Upload{
		ID:     hook.Upload.ID,
		UserID: null.IntFrom(user.ID),
		Size:   hook.Upload.Size,
		Status: models.UploadStatusCreated,
	}
	return upload.Insert(d.db, boil.Infer())
}

func (d *UploadsDB) markUploadProgress(ctx context.Context, hook handler.HookEvent, user *models.User) error {
	u, err := d.get(hook.Upload.ID, user.ID)
	if err != nil {
		return err
	}
	u.Received = hook.Upload.Offset
	u.Status = models.UploadStatusUploading
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(d.db, boil.Infer())
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return err
	}
	return nil
}

func (d *UploadsDB) startProcessingUpload(ctx context.Context, id string, user *models.User, path string, request *jsonrpc.RPCRequest) error {
	up, err := d.get(id, user.ID)
	if err != nil {
		return err
	}
	up.Status = models.UploadStatusReceived
	up.UpdatedAt = null.TimeFrom(time.Now())
	up.Path = path
	_, err = up.Update(d.db, boil.Infer())
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return err
	}

	req := null.JSON{}
	if err := req.Marshal(request); err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return err
	}
	uq := &models.Query{
		UpdatedAt: null.TimeFrom(time.Now()),
		Status:    models.QueryStatusReceived,
		Query:     req,
	}

	err = up.SetQuery(d.db, true, uq)
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return err
	}

	err = d.queue.AddUploadProcess(forklift.UploadProcessPayload{
		UploadID: up.ID,
		Path:     up.Path,
		UserID:   up.UserID.Int,
		Request:  request,
	})
	if err != nil {
		dbErr := d.markUploadFailed(ctx, up, err.Error())
		if dbErr != nil {
			return fmt.Errorf("enqueuing failed: %w (db update failed as well: %s)", err, dbErr)
		}
		return fmt.Errorf("enqueuing failed: %w", err)
	}
	return nil
}

func (d *UploadsDB) markUploadTerminated(ctx context.Context, hook handler.HookEvent, user *models.User) error {
	u, err := d.get(hook.Upload.ID, user.ID)
	if err != nil {
		return err
	}
	u.Status = models.UploadStatusTerminated
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(d.db, boil.Infer())
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return err
	}
	return nil
}

func (d *UploadsDB) markUploadFailed(ctx context.Context, u *models.Upload, e string) error {
	u.Error = e
	u.Status = models.UploadStatusFailed
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err := u.Update(d.db, boil.Infer())
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return err
	}
	return nil
}

func (d *UploadsDB) markUploadFinished(ctx context.Context, u *models.Upload) error {
	u.Status = models.UploadStatusFinished
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err := u.Update(d.db, boil.Infer())
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return err
	}
	err = os.RemoveAll(u.Path)
	if err != nil {
		metrics.Errors.WithLabelValues("storage").Inc()
		return err
	}
	return nil
}

func (d *UploadsDB) saveUploadProcessingResult(ctx context.Context, id string, rpcRes *jsonrpc.RPCResponse, callErr string) error {
	l := logging.FromContext(ctx)
	up, err := d.get(id, 0)
	if err != nil {
		l.Warn("error getting upload record", "err", err)
		metrics.Errors.WithLabelValues("db").Inc()
		return fmt.Errorf("error getting upload record: %w", err)
	}
	if rpcRes == nil {
		return d.markUploadFailed(ctx, up, callErr)
	}
	q := up.R.Query
	if q == nil {
		l.Warn("error getting upload query", "err", err)
		metrics.Errors.WithLabelValues("db").Inc()
		return errors.New("upload record is missing query")
	}
	resp := null.JSON{}
	if err := resp.Marshal(rpcRes); err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return err
	}
	q.UpdatedAt = null.TimeFrom(time.Now())
	q.Response = resp
	q.Error = callErr

	if rpcRes.Error != nil || callErr != "" {
		q.Status = models.QueryStatusFailed
	} else {
		q.Status = models.QueryStatusSucceeded
	}
	d.markUploadFinished(ctx, up)

	_, err = q.Update(d.db, boil.Infer())
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return fmt.Errorf("error updating upload record: %w", err)
	}
	return nil
}
