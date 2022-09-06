package geopublish

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/forklift"
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
			// case e := <-handler.CompleteUploads:
			// 	err = d.guardUser(d.markUploadReceived, d.db, e)
			case e = <-upHandler.TerminatedUploads:
				en = "TerminatedUploads"
				err := d.guardUser(d.markUploadTerminated, d.db, e)
				if err != nil {
					gerr = fmt.Errorf("error handling terminated upload signal: %w", err)
				}
				// case q := <-handler.preparedSDKQueries:
				// 	err = d.createQuery(d.db, q)
			}
			if gerr != nil {
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
		qm.Load(models.UploadRels.PublishQuery),
	}
	if userID > 0 {
		mods = append(mods, models.UploadWhere.UserID.EQ(null.IntFrom(userID)))
	}
	return models.Uploads(mods...).One(d.db)
}

func (d *UploadsDB) getQuery(id string) (*models.PublishQuery, error) {
	r, err := models.PublishQueries(
		models.PublishQueryWhere.UploadID.EQ(id),
	).One(d.db)
	if err != nil {
		return nil, fmt.Errorf("cannot get query with id=%s: %w", id, err)
	}
	return r, nil
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
	uq := &models.PublishQuery{
		UpdatedAt: null.TimeFrom(time.Now()),
		Status:    models.PublishQueryStatusReceived,
		Query:     req,
	}

	err = up.SetPublishQuery(d.db, true, uq)
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

// func (d *UploadsDB) markUploadProcessing(exec boil.Executor, hook handler.HookEvent, user *models.User) error {
// 	u, err := d.get(hook.Upload.ID, user.ID)
// 	if err != nil {
// 		return err
// 	}
// 	u.Status = models.UploadStatusProcessing
// 	u.UpdatedAt = null.TimeFrom(time.Now())
// 	_, err = u.Update(exec, boil.Infer())
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

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

// func (d *UploadsDB) getPendingQueries() (models.PublishQuerySlice, error) {
// 	q := models.PublishQueries(
// 		qm.Where(
// 			"(status=?) or (status=? and retries lt ?)",
// 			models.PublishQueryStatusReceived,
// 			models.PublishQueryStatusFailed,
// 			queryMaxRetries,
// 		),
// 		qm.OrderBy(models.PublishQueryColumns.UpdatedAt),
// 		qm.Load(models.PublishQueryRels.Upload),
// 	)
// 	return q.All(d.db)
// }

// func (d *UploadsDB) markQueryForwarded(exec boil.Executor, id string) (*models.PublishQuery, error) {
// 	q, err := d.getQuery(id)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if q.Status == models.PublishQueryStatusFailed {
// 		q.Retries += 1
// 	}
// 	q.Status = models.PublishQueryStatusForwarded
// 	q.UpdatedAt = null.TimeFrom(time.Now())
// 	_, err = q.Update(exec, boil.Infer())
// 	if err != nil {
// 		return nil, err
// 	}
// 	return q, nil
// }

func (d *UploadsDB) saveUploadProcessingResult(id string, rpcRes *jsonrpc.RPCResponse, callErr string) error {
	up, err := d.get(id, 0)
	if err != nil {
		return err
	}
	if rpcRes == nil {
		return d.markUploadFailed(up, callErr)
	}
	q, err := d.getQuery(id)
	if err != nil {
		return err
	}
	resp := null.JSON{}
	if err := resp.Marshal(rpcRes); err != nil {
		return err
	}
	q.UpdatedAt = null.TimeFrom(time.Now())
	q.Response = resp
	q.Error = callErr

	if rpcRes.Error != nil || callErr != "" {
		q.Status = models.PublishQueryStatusFailed
	} else {
		q.Status = models.PublishQueryStatusSucceeded
	}
	d.markUploadFinished(up)

	_, err = q.Update(d.db, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}

// func (d *UploadsDB) uploadCompletedQuery(exec boil.Executor, q uploadCompleteddQuery) error {
// 	u, err := d.get(q.fileInfo.ID)
// 	if err != nil {
// 		return err
// 	}
// 	if q.response.Error != nil || q.err != nil {
// 		u.Status = models.UploadStatusQueryFailed
// 	} else {
// 		u.Status = models.UploadStatusCompleted
// 	}
// 	u.UpdatedAt = null.TimeFrom(time.Now())
// 	_, err = u.Update(exec, boil.Infer())
// 	if err != nil {
// 		return err
// 	}

// 	resp := null.JSON{}
// 	if err := resp.Marshal(q.response); err != nil {
// 		return err
// 	}
// 	uq := u.R.UploadQuery
// 	uq.Response = resp
// 	uq.Error = q.err.Error()
// 	_, err = uq.Update(exec, boil.Infer())
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
