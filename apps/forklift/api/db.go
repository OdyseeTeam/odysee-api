package api

import (
	"context"
	"fmt"
	"net/http"
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

func (d *UploadsDB) get(id string, userID int) (*models.ForkliftUpload, error) {
	mods := []qm.QueryMod{
		models.ForkliftUploadWhere.ID.EQ(id),
	}
	if userID > 0 {
		mods = append(mods, models.ForkliftUploadWhere.UserID.EQ(null.IntFrom(userID)))
	}
	return models.ForkliftUploads(mods...).One(d.db)
}

func (d *UploadsDB) markUploadCreated(ctx context.Context, hook tus.HookEvent, user *models.User) error {
	up := models.ForkliftUpload{
		ID:     hook.Upload.ID,
		UserID: null.IntFrom(user.ID),
		Size:   hook.Upload.Size,
		Status: models.ForkliftUploadStatusCreated,
	}
	return up.Insert(d.db, boil.Infer())
}

func (d *UploadsDB) markUploadProgress(ctx context.Context, hook tus.HookEvent, user *models.User) error {
	u, err := d.get(hook.Upload.ID, user.ID)
	if err != nil {
		return err
	}
	u.Received = hook.Upload.Offset
	u.Status = models.ForkliftUploadStatusUploading
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(d.db, boil.Infer())
	if err != nil {
		metricErrors.WithLabelValues(areaDB).Inc()
		return err
	}
	return nil
}

func (d *UploadsDB) startProcessingUpload(ctx context.Context, id string, user *models.User, path string) (*models.ForkliftUpload, error) {
	up, err := models.ForkliftUploads(qm.Where("id = ? and user_id = ? and status = ?", id, user.ID, models.ForkliftUploadStatusUploading)).One(d.db)
	if err != nil {
		return nil, err
	}
	up.Status = models.ForkliftUploadStatusReceived
	up.UpdatedAt = null.TimeFrom(time.Now())
	up.Path = path
	_, err = up.Update(d.db, boil.Infer())
	if err != nil {
		metricErrors.WithLabelValues(areaDB).Inc()
		return nil, err
	}

	return up, nil
}

func (d *UploadsDB) markUploadTerminated(ctx context.Context, hook tus.HookEvent, user *models.User) error {
	up, err := models.ForkliftUploads(qm.Where("id = ? and user_id = ? and status in ?", hook.Upload.ID, user.ID,
		models.ForkliftUploadStatusCreated, models.ForkliftUploadStatusUploading, models.ForkliftUploadStatusReceived)).One(d.db)
	if err != nil {
		return err
	}
	up.Status = models.ForkliftUploadStatusTerminated
	up.UpdatedAt = null.TimeFrom(time.Now())
	_, err = up.Update(d.db, boil.Infer())
	if err != nil {
		metricErrors.WithLabelValues(areaDB).Inc()
		return err
	}
	return nil
}

func (d *UploadsDB) markUploadFailed(ctx context.Context, u *models.ForkliftUpload, errMessage string) error {
	u.Error = errMessage
	u.Status = models.ForkliftUploadStatusFailed
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err := u.Update(d.db, boil.Infer())
	if err != nil {
		metricErrors.WithLabelValues(areaDB).Inc()
		return err
	}
	return nil
}

func (d *UploadsDB) markUploadFinished(ctx context.Context, u *models.ForkliftUpload) error {
	u.Status = models.ForkliftUploadStatusFinished
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err := u.Update(d.db, boil.Infer())
	if err != nil {
		metricErrors.WithLabelValues(areaDB).Inc()
		return err
	}
	return nil
}

func (d *UploadsDB) finishUpload(ctx context.Context, id string, rpcRes *jsonrpc.RPCResponse, callErr string) error {
	l := logging.GetFromContext(ctx)
	up, err := d.get(id, 0)
	if err != nil {
		l.Warn("error getting upload record", "err", err)
		metricErrors.WithLabelValues(areaDB).Inc()
		return fmt.Errorf("error getting upload record: %w", err)
	}
	if rpcRes == nil {
		return d.markUploadFailed(ctx, up, callErr)
	}

	if rpcRes.Error != nil || callErr != "" {
		return d.markUploadFailed(ctx, up, rpcRes.Error.Error())
	}
	return d.markUploadFinished(ctx, up)
}
