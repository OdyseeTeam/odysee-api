package publish

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/query/cache"
	"github.com/lbryio/lbrytv/app/rpcerrors"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/ybbus/jsonrpc"
)

const module = "publish.tus"

// TusHandler handle media publishing on odysee-api, it implements TUS
// specifications to support resumable file upload and extends the handler to
// support fetching media from remote url.
type TusHandler struct {
	*tusd.UnroutedHandler

	logger       monitor.ModuleLogger
	composer     *tusd.StoreComposer
	authProvider auth.Provider
}

// NewTusHandler creates a new publish handler.
func NewTusHandler(authProvider auth.Provider, cfg tusd.Config, uploadPath string) (*TusHandler, error) {
	h := &TusHandler{}

	if authProvider == nil {
		return nil, fmt.Errorf("auth provider cannot be nil")
	}

	defaultUploadPath := "./uploads"
	if uploadPath == "" {
		uploadPath = defaultUploadPath
	}
	if err := os.MkdirAll(uploadPath, os.ModePerm); err != nil {
		return nil, err
	}

	cfg.PreUploadCreateCallback = h.preCreateHook
	// allow client to set location response protocol
	// via X-Forwarded-Proto
	cfg.RespectForwardedHeaders = true

	handler, err := tusd.NewUnroutedHandler(cfg)
	if err != nil {
		return nil, err
	}

	h.UnroutedHandler = handler
	h.logger = monitor.NewModuleLogger(module)
	h.authProvider = authProvider
	h.composer = cfg.StoreComposer

	return h, nil
}

// Notify checks if the file upload is complete and sends jSON RPC request to lbrynet server.
func (h TusHandler) Notify(w http.ResponseWriter, r *http.Request) {
	log := h.logger.WithFields(
		logrus.Fields{
			"method_handler": "Notify",
		},
	)

	user, err := auth.FromRequest(r)
	if authErr := proxy.GetAuthError(user, err); authErr != nil {
		log.WithError(authErr).Error("failed to authorize user")
		w.Write(rpcerrors.ErrorToJSON(authErr))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindAuth)
		return
	}
	log = log.WithField("user_id", user.ID)

	if sdkrouter.GetSDKAddress(user) == "" {
		log.Errorf("user %d does not have sdk address assigned", user.ID)
		w.Write(rpcerrors.NewInternalError(errors.Err("user does not have sdk address assigned")).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
		return
	}

	params := mux.Vars(r)
	id := params["id"]
	if id == "" {
		err := fmt.Errorf("file id is required")
		log.Error(err)
		w.Write(rpcerrors.NewInvalidParamsError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindClient)
		return
	}

	if h.composer.UsesLocker {
		lock, err := h.lockUpload(id)
		if err != nil {
			log.WithError(err).Error("failed to acquire file lock")
			w.Write(rpcerrors.NewInternalError(err).JSON())
			observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
			return
		}
		defer lock.Unlock()
	}

	upload, err := h.composer.Core.GetUpload(r.Context(), id)
	if err != nil {
		log.WithError(err).Error("failed to get upload object")
		w.Write(rpcerrors.NewInternalError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindClient)
		return
	}

	info, err := upload.GetInfo(r.Context())
	if err != nil {
		log.WithError(err).Error("failed to get upload info")
		w.Write(rpcerrors.NewInternalError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
		return
	}

	// NOTE: don't use info.IsFinal as it's not reflect the upload
	// completion at all
	if info.Offset != info.Size { // upload is not yet completed
		err := fmt.Errorf("upload is still in process")
		log.WithError(err).Error("file incomplete")
		w.Write(rpcerrors.ErrorToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindClient)
		return
	}

	// uploadMD holds uploaded file metadata sent by client when it first
	// start the upload sequence.
	uploadMD := info.MetaData
	if len(uploadMD) == 0 {
		err := fmt.Errorf("file metadata is required")
		log.Error(err.Error())
		w.Write(rpcerrors.ErrorToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindClient)
		return
	}

	origUploadName, ok := uploadMD["filename"]
	if !ok || origUploadName == "" {
		err := fmt.Errorf("file name is required")
		log.Error(err.Error())
		w.Write(rpcerrors.ErrorToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindClient)
		return

	}

	origUploadPath, ok := info.Storage["Path"]
	if !ok || origUploadPath == "" { // shouldn't happen but check regardless
		log.Errorf("file path property not found in storage info: %v", reflect.ValueOf(info.Storage).MapKeys())
		w.Write(rpcerrors.ErrorToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
		return
	}

	// ignore the file name and rename the uploaded file to the new location
	// with name based on the value from upload metadata.
	dir, _ := filepath.Split(origUploadPath)

	dstDir := filepath.Join(dir, strconv.Itoa(user.ID))
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		log.WithError(err).Errorf("failed to create directory: %s", dstDir)
		w.Write(rpcerrors.ErrorToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
		return
	}

	dstFilepath := filepath.Join(dstDir, origUploadName)
	if err := os.Rename(origUploadPath, dstFilepath); err != nil {
		log.WithError(err).Errorf("failed to rename uploaded file to: %s", dstFilepath)
		w.Write(rpcerrors.ErrorToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
		return
	}

	// upload is completed, notify it to lbrynet server
	var qCache cache.QueryCache
	if cache.IsOnRequest(r) {
		qCache = cache.FromRequest(r)
	}

	var rpcReq jsonrpc.RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpcReq); err != nil {
		w.Write(rpcerrors.NewJSONParseError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindClientJSON)
		return
	}

	c := getCaller(sdkrouter.GetSDKAddress(user), dstFilepath, user.ID, qCache)

	op := metrics.StartOperation("sdk", "call_publish")
	rpcRes, err := c.Call(&rpcReq)
	op.End()
	if err != nil {
		monitor.ErrorToSentry(
			fmt.Errorf("error calling publish: %v", err),
			map[string]string{
				"request":  fmt.Sprintf("%+v", rpcReq),
				"response": fmt.Sprintf("%+v", rpcRes),
			},
		)
		log.WithError(err).Errorf("error calling publish, request: %+v", rpcReq)
		w.Write(rpcerrors.ToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindRPC)
		return
	}

	serialized, err := responses.JSONRPCSerialize(rpcRes)
	if err != nil {
		log.WithError(err).Error("error marshalling response")
		monitor.ErrorToSentry(err)
		w.Write(rpcerrors.NewInternalError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindRPCJSON)
		return
	}

	// NOTE: DO NOT use store.Terminate to remove the uploaded files from tusd package
	// as it will fail since we rename the file previously.
	infoFile := fmt.Sprintf("%s.info",
		filepath.Join(dir, info.ID),
	)
	if err := os.Remove(infoFile); err != nil {
		log.WithError(err).Error("failed to remove upload info file")
		monitor.ErrorToSentry(err, map[string]string{"info_file": infoFile})
	}
	if err := os.RemoveAll(dstDir); err != nil {
		log.WithError(err).Error("failed to remove file")
		monitor.ErrorToSentry(err, map[string]string{"file_path": dstFilepath})
	}

	w.Write(serialized)
	observeSuccess(metrics.GetDuration(r))
}

func (h TusHandler) lockUpload(id string) (tusd.Lock, error) {
	lock, err := h.composer.Locker.NewLock(id)
	if err != nil {
		return nil, err
	}
	if err := lock.Lock(); err != nil {
		return nil, err
	}
	return lock, nil
}

// preCreateHook validates user access request to publish handler before we
// attempt to start the upload procedures.
//
// Note that usually this should be done as part of http middleware, but TUS
// handlers overwrite the context with context background to avoid context
// cancellation, and so any attempt to read values from request context won't
// work here, until they can safely pass request context to TUS handler we need
// to decouple before and after middleware to TUS hook callback functions.
//
// see: https://github.com/tus/tusd/pull/342
func (h *TusHandler) preCreateHook(hook tusd.HookEvent) error {
	log := h.logger.WithFields(
		logrus.Fields{
			"method_handler": "preCreateHook",
		},
	)

	r := hook.HTTPRequest
	user, err := userFromRequest(h.authProvider, r.Header, r.RemoteAddr)
	if err != nil {
		log.WithError(err).Info("error authenticating user")
		return err
	}
	if user == nil {
		err := auth.ErrNoAuthInfo
		log.WithError(err).Info("unauthorized user")
		return err
	}
	return nil
}
