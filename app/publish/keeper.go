package publish

import (
	"time"

	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/tus/tusd/pkg/handler"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	// . "github.com/volatiletech/sqlboiler/queries/qm"
)

type UploadKeeper struct {
	handler *TusHandler
	db      boil.Executor
}

func (d *UploadKeeper) listenToHandler(handler *TusHandler) {
	d.handler = handler

	go func() {
		for {
			var err error
			select {
			case e := <-handler.CreatedUploads:
				err = d.create(d.db, e)
			case e := <-handler.UploadProgress:
				err = d.updateProgress(d.db, e)
			case e := <-handler.TerminatedUploads:
				err = d.terminate(d.db, e)
			case q := <-handler.preparedSDKQueries:
				err = d.createQuery(d.db, q)
			case q := <-handler.completedSDKQueries:
				err = d.completeQuery(d.db, q)
			}
			if err != nil {
				// LOG
			}
		}

	}()
}

func (d *UploadKeeper) get(id string) (*models.Upload, error) {
	return models.Uploads(
		models.UploadWhere.ID.EQ(id),
		// Load(models.UploadRels.UploadQuery),
	).One(d.db)
}

func (d *UploadKeeper) create(exec boil.Executor, hook handler.HookEvent) error {
	user, err := d.handler.multiAuthUser(hook.HTTPRequest.Header, hook.HTTPRequest.RemoteAddr)
	if err != nil {
		return err
	}
	upload := models.Upload{
		ID:     hook.Upload.ID,
		UserID: null.IntFrom(user.ID),
		Size:   hook.Upload.Size,
		Status: models.UploadStatusCreated,
	}
	return upload.Insert(exec, boil.Infer())
}

func (d *UploadKeeper) updateProgress(exec boil.Executor, hook handler.HookEvent) error {
	u, err := d.get(hook.Upload.ID)
	if err != nil {
		return err
	}
	u.Received = hook.Upload.Offset
	u.Status = models.UploadStatusUploading
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(exec, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}

func (d *UploadKeeper) complete(exec boil.Executor, hook handler.HookEvent) error {
	u, err := d.get(hook.Upload.ID)
	if err != nil {
		return err
	}
	u.Received = hook.Upload.Offset
	u.Status = models.UploadStatusUploaded
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(exec, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}

func (d *UploadKeeper) terminate(exec boil.Executor, hook handler.HookEvent) error {
	u, err := d.get(hook.Upload.ID)
	if err != nil {
		return err
	}
	u.Status = models.UploadStatusTerminated
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(exec, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}

func (d *UploadKeeper) createQuery(exec boil.Executor, q preparedQuery) error {
	u, err := d.get(q.fileInfo.ID)
	if err != nil {
		return err
	}
	u.Status = models.UploadStatusQuerySent
	u.UpdatedAt = null.TimeFrom(time.Now())

	req := null.JSON{}
	if err := req.Marshal(q.request); err != nil {
		return err
	}
	_, err = u.Update(exec, boil.Infer())
	if err != nil {
		return err
	}
	err = u.SetUploadQuery(exec, true, &models.UploadQuery{
		Query: req,
	})
	if err != nil {
		return err
	}
	return nil
}

func (d *UploadKeeper) completeQuery(exec boil.Executor, q completedQuery) error {
	u, err := d.get(q.fileInfo.ID)
	if err != nil {
		return err
	}
	if q.response.Error != nil || q.err != nil {
		u.Status = models.UploadStatusQueryFailed
	} else {
		u.Status = models.UploadStatusCompleted
	}
	u.UpdatedAt = null.TimeFrom(time.Now())
	_, err = u.Update(exec, boil.Infer())
	if err != nil {
		return err
	}

	resp := null.JSON{}
	if err := resp.Marshal(q.response); err != nil {
		return err
	}
	uq := u.R.UploadQuery
	uq.Response = resp
	uq.Error = q.err.Error()
	_, err = uq.Update(exec, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}
