package publish

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/proxy"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/query/cache"
	"github.com/OdyseeTeam/odysee-api/app/rpcerrors"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/internal/responses"
	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/gorilla/mux"
	werrors "github.com/pkg/errors"
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

	uploadPath   string
	logger       monitor.ModuleLogger
	composer     *tusd.StoreComposer
	authProvider auth.Provider
	auther       auth.Authenticator
}

// NewTusHandler creates a new publish handler.
// Auther is required because of the way tus handles http requests, see preCreateHook.
// Provider is a temporary mechanism for supporting authentication with legacy tokens.
// TODO: Remove auth.Provider after legacy tokens go away.
func NewTusHandler(auther auth.Authenticator, provider auth.Provider, cfg tusd.Config, uploadPath string) (*TusHandler, error) {
	h := &TusHandler{}

	if auther == nil {
		return nil, fmt.Errorf("authenticator is required")
	}
	if provider == nil {
		return nil, fmt.Errorf("legacy auth provider is required")
	}

	defaultUploadPath := "./uploads"
	if uploadPath == "" {
		uploadPath = defaultUploadPath
	}
	if err := os.MkdirAll(uploadPath, os.ModePerm); err != nil {
		return nil, err
	}
	h.uploadPath = uploadPath

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
	h.authProvider = provider
	h.auther = auther
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

	user, err := h.multiAuthUser(r)
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
			observeFailure(metrics.GetDuration(r), metrics.PublishLockFailure)
			return
		}
		defer lock.Unlock()
	}

	upload, err := h.composer.Core.GetUpload(r.Context(), id)
	if err != nil {
		log.WithError(err).Error("failed to get upload object")
		w.Write(rpcerrors.NewInternalError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.PublishUploadObjectFailure)
		return
	}

	info, err := upload.GetInfo(r.Context())
	if err != nil {
		log.WithError(err).Error("failed to get upload info")
		w.Write(rpcerrors.NewInternalError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.PublishUploadObjectFailure)
		return
	}

	// NOTE: don't use info.IsFinal as it's not reflect the upload
	// completion at all
	if info.Offset != info.Size { // upload is not yet completed
		err := fmt.Errorf("upload is still in process")
		log.WithError(err).Error("file incomplete")
		w.Write(rpcerrors.ErrorToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.PublishUploadIncomplete)
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

	// rename the uploaded file to the new location
	// with name based on the value from upload metadata.
	dir := filepath.Dir(origUploadPath)

	dstDir := filepath.Join(dir, strconv.Itoa(user.ID), info.ID)
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
	var qCache *cache.Cache
	if cache.IsOnRequest(r) {
		qCache = cache.FromRequest(r)
	}

	var rpcReq *jsonrpc.RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpcReq); err != nil {
		w.Write(rpcerrors.NewJSONParseError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindClientJSON)
		return
	}

	rpcparams, ok := rpcReq.Params.(map[string]interface{})
	if !ok {
		w.Write(rpcerrors.NewInvalidParamsError(werrors.New("cannot parse params")).JSON())
		return
	}

	if rpcparams["claim_id"] != nil {
		rpcReq.Method = query.MethodStreamUpdate
		delete(rpcparams, "name")
		rpcparams["replace"] = true
		rpcReq.Params = rpcparams
	}

	c := getCaller(sdkrouter.GetSDKAddress(user), dstFilepath, user.ID, qCache)

	op := metrics.StartOperation("sdk", "call_publish")
	rpcRes, err := c.Call(rpcReq)
	defer op.End()
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
	r := &http.Request{
		Header: hook.HTTPRequest.Header,
	}
	_, err := h.multiAuthUser(r)
	return err
}

func (h *TusHandler) multiAuthUser(r *http.Request) (*models.User, error) {
	log := h.logger.Log()
	token, err := h.auther.GetTokenFromRequest(r)
	if errors.Is(err, wallet.ErrNoAuthInfo) {
		// TODO: Remove this pathway after legacy tokens go away.
		if token, ok := r.Header[wallet.LegacyTokenHeader]; ok {
			addr := ip.AddressForRequest(r.Header, r.RemoteAddr)
			user, err := h.authProvider(token[0], addr)
			if err != nil {
				log.WithError(err).Info("error authenticating user")
				return nil, err
			}
			if user == nil {
				err := wallet.ErrNoAuthInfo
				log.WithError(err).Info("unauthorized user")
				return nil, err
			}
			return user, nil
		}
		return nil, errors.Err(wallet.ErrNoAuthInfo)
	} else if err != nil {
		return nil, err
	}

	user, err := h.auther.Authenticate(token, ip.AddressForRequest(r.Header, r.RemoteAddr))
	if err != nil {
		log.WithError(err).Info("error authenticating user")
		return nil, err
	}
	if user == nil {
		err := wallet.ErrNoAuthInfo
		log.WithError(err).Info("unauthorized user")
		return nil, err
	}
	return user, nil
}
