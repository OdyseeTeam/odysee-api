package geopublish

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/forklift"
	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/models"

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
	logger  monitor.ModuleLogger
}

type markFunc func(hook handler.HookEvent, user *models.User) error

func (d *UploadsDB) guardUser(mf markFunc, exec boil.Executor, hook handler.HookEvent) error {
	user, err := d.handler.getUserFromRequest(&http.Request{
		Header: hook.HTTPRequest.Header,
	})
	if err != nil {
		return err
	}
	return mf(hook, user)
}

func (d *UploadsDB) listenToHandler(upHandler *Handler) {
	d.handler = upHandler
	log := d.logger.Log()

	go func() {
		for {
			log.Debug("getting upload result from queue")
			res, err := d.queue.GetUploadProcessResult()
			if err != nil {
				log.Errorf("error getting result: %s", err)
				continue
			}
			log.Debugf("got upload result: %+v", res)
			err = d.saveUploadProcessingResult(res.UploadID, res.Response, res.Error)
			if err != nil {
				log.Errorf("error saving query result: %s", err)
			} else {
				metrics.UploadsProcessed.Inc()
				log.Infof("upload %s processed", res.UploadID)
			}
		}
	}()

	go func() {
		for {
			var (
				gerr error
				en   string
				e    handler.HookEvent
			)
			log.Infof("listening to handler")
			select {
			case e = <-upHandler.CreatedUploads:
				metrics.UploadsCreated.Inc()
				en = "CreatedUploads"
				err := d.guardUser(d.markUploadCreated, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling created uploads signal: %w", err)
				}
			case e = <-upHandler.UploadProgress:
				en = "UploadProgress"
				err := d.guardUser(d.markUploadProgress, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling upload progress signal: %w", err)
				}
			case e = <-upHandler.TerminatedUploads:
				metrics.UploadsCanceled.Inc()
				en = "TerminatedUploads"
				err := d.guardUser(d.markUploadTerminated, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling terminated upload signal: %w", err)
				}
			}
			if gerr != nil {
				metrics.UploadsDBErrors.Inc()
				log.Error(gerr)
			} else {
				log.Infof("handled %s signal, upload id=%s", en, e.Upload.ID)
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

func (d *UploadsDB) markUploadCreated(hook handler.HookEvent, user *models.User) error {
	upload := models.Upload{
		ID:     hook.Upload.ID,
		UserID: null.IntFrom(user.ID),
		Size:   hook.Upload.Size,
		Status: models.UploadStatusCreated,
	}
	return upload.Insert(d.db, boil.Infer())
}

func (d *UploadsDB) markUploadProgress(hook handler.HookEvent, user *models.User) error {
	u, err := d.get(hook.Upload.ID, user.ID)
	if err != nil {
		return err
	}
	u.Received = hook.Upload.Offset
	u.Status = models.UploadStatusUploading
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(d.db, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}

func (d *UploadsDB) processUpload(id string, user *models.User, path string, request *jsonrpc.RPCRequest) error {
	up, err := d.get(id, user.ID)
	if err != nil {
		return err
	}
	up.Status = models.UploadStatusReceived
	up.UpdatedAt = null.TimeFrom(time.Now())
	up.Path = path
	_, err = up.Update(d.db, boil.Infer())
	if err != nil {
		return err
	}

	req := null.JSON{}
	if err := req.Marshal(request); err != nil {
		return err
	}
	uq := &models.Query{
		UpdatedAt: null.TimeFrom(time.Now()),
		Status:    models.QueryStatusReceived,
		Query:     req,
	}

	err = up.SetQuery(d.db, true, uq)
	if err != nil {
		return err
	}

	err = d.queue.EnqueueUploadProcessTask(forklift.UploadProcessPayload{
		UploadID: up.ID,
		Path:     up.Path,
		UserID:   up.UserID.Int,
		Request:  request,
	})
	if err != nil {
		dbErr := d.markUploadFailed(up, err.Error())
		if dbErr != nil {
			return fmt.Errorf("enqueuing failed: %w (db update failed as well: %s)", err, dbErr)
		}
		return fmt.Errorf("enqueuing failed: %w", err)
	}
	return nil
}

func (d *UploadsDB) markUploadTerminated(hook handler.HookEvent, user *models.User) error {
	u, err := d.get(hook.Upload.ID, user.ID)
	if err != nil {
		return err
	}
	u.Status = models.UploadStatusTerminated
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(d.db, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}

func (d *UploadsDB) markUploadFailed(u *models.Upload, e string) error {
	u.Error = e
	u.Status = models.UploadStatusFailed
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err := u.Update(d.db, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}

func (d *UploadsDB) markUploadFinished(u *models.Upload) error {
	u.Status = models.UploadStatusFinished
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err := u.Update(d.db, boil.Infer())
	if err != nil {
		return err
	}
	err = os.RemoveAll(u.Path)
	if err != nil {
		return err
	}
	return nil
}

func (d *UploadsDB) saveUploadProcessingResult(id string, rpcRes *jsonrpc.RPCResponse, callErr string) error {
	up, err := d.get(id, 0)
	if err != nil {
		return err
	}
	if rpcRes == nil {
		return d.markUploadFailed(up, callErr)
	}
	q := up.R.Query
	if q == nil {
		return errors.New("upload record is missing query")
	}
	resp := null.JSON{}
	if err := resp.Marshal(rpcRes); err != nil {
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
	d.markUploadFinished(up)

	_, err = q.Update(d.db, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}
