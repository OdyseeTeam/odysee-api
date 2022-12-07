package geopublish

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	tus "github.com/tus/tusd/pkg/handler"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"
)

type UploadsDB struct {
	handler *Handler
	db      boil.Executor
	logger  logging.KVLogger
}

type markFunc func(ctx context.Context, hook tus.HookEvent, user *models.User) error

func (d *UploadsDB) guardUser(ctx context.Context, mf markFunc, exec boil.Executor, hook tus.HookEvent) error {
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
	log := d.logger

	go func() {
		for {
			var (
				gerr  error
				ename string
				e     tus.HookEvent
			)
			l := d.logger.With("upload_id", e.Upload.ID)
			ctx := logging.AddToContext(context.Background(), l)
			log.Debug("listening to handler")
			select {
			case e = <-upHandler.CreatedUploads:
				metrics.UploadsCreated.Inc()
				ename = "CreatedUploads"
				err := d.guardUser(ctx, d.markUploadCreated, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling created uploads signal: %w", err)
				}
			case e = <-upHandler.UploadProgress:
				ename = "UploadProgress"
				err := d.guardUser(ctx, d.markUploadProgress, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling upload progress signal: %w", err)
				}
			case e = <-upHandler.TerminatedUploads:
				metrics.UploadsCanceled.Inc()
				ename = "TerminatedUploads"
				err := d.guardUser(ctx, d.markUploadTerminated, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling terminated upload signal: %w", err)
				}
			}
			if gerr != nil {
				l.Warn("upload signal error", "err", gerr)
			} else {
				l.Debug("handled upload signal", "event_name", ename)
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

func (d *UploadsDB) markUploadCreated(ctx context.Context, hook tus.HookEvent, user *models.User) error {
	upload := models.Upload{
		ID:     hook.Upload.ID,
		UserID: null.IntFrom(user.ID),
		Size:   hook.Upload.Size,
		Status: models.UploadStatusCreated,
	}
	return upload.Insert(d.db, boil.Infer())
}

func (d *UploadsDB) markUploadProgress(ctx context.Context, hook tus.HookEvent, user *models.User) error {
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

func (d *UploadsDB) startProcessingUpload(ctx context.Context, id string, user *models.User, path string) (*models.Upload, error) {
	up, err := d.get(id, user.ID)
	if err != nil {
		return nil, err
	}
	up.Status = models.UploadStatusReceived
	up.UpdatedAt = null.TimeFrom(time.Now())
	up.Path = path
	_, err = up.Update(d.db, boil.Infer())
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		return nil, err
	}

	return up, nil
}

func (d *UploadsDB) markUploadTerminated(ctx context.Context, hook tus.HookEvent, user *models.User) error {
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

func (d *UploadsDB) markUploadFailed(ctx context.Context, u *models.Upload, errMessage string) error {
	u.Error = errMessage
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

func (d *UploadsDB) finishUpload(ctx context.Context, id string, rpcRes *jsonrpc.RPCResponse, callErr string) error {
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
