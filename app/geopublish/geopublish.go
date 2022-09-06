package geopublish

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/forklift"
	"github.com/OdyseeTeam/odysee-api/app/proxy"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/query/cache"
	"github.com/OdyseeTeam/odysee-api/app/rpcerrors"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/volatiletech/sqlboiler/boil"

	"github.com/gorilla/mux"
	werrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/ybbus/jsonrpc"
)

var logger = monitor.NewModuleLogger("geopublish")

const (
	// fileFieldName refers to the POST field containing file upload
	fileFieldName = "file"
	// jsonRPCFieldName is a name of the POST field containing JSONRPC request accompanying the uploaded file
	jsonRPCFieldName = "json_payload"
	opName           = "publish"
	fileNameParam    = "file_path"
	remoteURLParam   = "remote_url"
	method           = "publish"
	module           = "geopublish"
)

type UserGetter interface {
	GetFromRequest(*http.Request) (*models.User, error)
}

type preparedQuery struct {
	request  *jsonrpc.RPCRequest
	fileInfo tusd.FileInfo
}

type completedQuery struct {
	response *jsonrpc.RPCResponse
	err      error
	fileInfo tusd.FileInfo
}

// Handler handle media publishing on odysee-api, it implements TUS
// specifications to support resumable file upload and extends the handler to
// support fetching media from remote url.
type Handler struct {
	*tusd.UnroutedHandler

	options  *HandlerOptions
	composer *tusd.StoreComposer
	logger   monitor.ModuleLogger
	udb      *UploadsDB

	preparedSDKQueries chan preparedQuery
}

type HandlerOptions struct {
	// Logger   logging.KVLogger
	userGetter UserGetter
	uploadPath string
	tusConfig  *tusd.Config
	db         boil.Executor
	queue      *forklift.Forklift
}

// func WithLogger(logger logging.KVLogger) func(options *HandlerOptions) {
// 	return func(options *HandlerOptions) {
// 		options.Logger = logger
// 	}
// }

// CanHandle checks if http.Request contains POSTed data in an accepted format.
// Supposed to be used in gorilla mux router MatcherFunc.
func CanHandle(r *http.Request, _ *mux.RouteMatch) bool {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		return false
	}
	return r.FormValue(jsonRPCFieldName) != ""
}

func WithUploadPath(uploadPath string) func(options *HandlerOptions) {
	return func(options *HandlerOptions) {
		options.uploadPath = uploadPath
	}
}

// WithAuther is required because of the way tus handles http requests, see preCreateHook.
func WithUserGetter(userGetter UserGetter) func(options *HandlerOptions) {
	return func(options *HandlerOptions) {
		options.userGetter = userGetter
	}
}

func WithDB(db boil.Executor) func(options *HandlerOptions) {
	return func(options *HandlerOptions) {
		options.db = db
	}
}

func WithQueue(queue *forklift.Forklift) func(options *HandlerOptions) {
	return func(options *HandlerOptions) {
		options.queue = queue
	}
}

func WithTusConfig(config tusd.Config) func(options *HandlerOptions) {
	return func(options *HandlerOptions) {
		options.tusConfig = &config
	}
}

// NewHandler creates a new publish handler.

func NewHandler(optionFuncs ...func(*HandlerOptions)) (*Handler, error) {
	options := &HandlerOptions{
		// Logger: logging.NoopKVLogger{},
		uploadPath: "./uploads",
		tusConfig:  &tusd.Config{},
	}
	for _, optionFunc := range optionFuncs {
		optionFunc(options)
	}

	h := &Handler{options: options}

	if options.userGetter == nil {
		return nil, fmt.Errorf("user getter is required")
	}

	if err := os.MkdirAll(options.uploadPath, os.ModePerm); err != nil {
		return nil, err
	}

	cfg := options.tusConfig

	cfg.PreUploadCreateCallback = h.preCreateHook
	// allow client to set location response protocol
	// via X-Forwarded-Proto
	cfg.RespectForwardedHeaders = true

	h.logger = monitor.NewModuleLogger(module)
	udb := UploadsDB{logger: h.logger, db: options.db, queue: options.queue}
	udb.listenToHandler(h)
	// cfg.NotifyCompleteUploads = true
	cfg.NotifyCreatedUploads = true
	cfg.NotifyTerminatedUploads = true
	cfg.NotifyUploadProgress = true

	h.udb = &udb

	baseHandler, err := tusd.NewUnroutedHandler(*cfg)
	if err != nil {
		return nil, err
	}

	h.UnroutedHandler = baseHandler
	h.composer = cfg.StoreComposer

	return h, nil
}

// Notify checks if the file upload is complete and sends jSON RPC request to lbrynet server.
func (h Handler) Notify(w http.ResponseWriter, r *http.Request) {
	log := h.logger.WithFields(
		logrus.Fields{
			"method_handler": "Notify",
		},
	)

	user, err := h.getUserFromRequest(r)
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

	lock, err := h.lockUpload(id)
	if err != nil {
		monitor.ErrorToSentry(err, map[string]string{
			"upload_id": id,
			"user_id":   strconv.Itoa(user.ID),
		})
		log.WithError(err).Error("failed to acquire file lock")
		w.Write(rpcerrors.NewInternalError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.PublishLockFailure)
		return
	}
	defer lock.Unlock()

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
		log.WithError(err).Error("upload is still in process")
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

	// NOTE: DO NOT use store.Terminate to remove the uploaded files from tusd package
	// as it will fail since we rename the file previously.
	infoFile := fmt.Sprintf("%s.info",
		filepath.Join(dir, info.ID),
	)
	if err := os.Remove(infoFile); err != nil {
		log.WithError(err).Error("failed to remove upload info file")
		monitor.ErrorToSentry(err, map[string]string{"info_file": infoFile})
	}

	err = h.udb.processUpload(info.ID, user, dstFilepath, rpcReq)
	if err != nil {
		log.WithError(err).Error("upload processing failed")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// IsReady returns status of the upload.
func (h Handler) Status(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]
	if id == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	user, err := h.getUserFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	up, err := h.udb.get(id, user.ID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pq, err := up.PublishQuery().One(h.udb.db)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(err.Error()))
		return
	}

	switch pq.Status {
	case models.PublishQueryStatusSucceeded:
		w.WriteHeader(http.StatusOK)
		w.Write(pq.Response.JSON)
	case models.PublishQueryStatusFailed:
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(pq.Error))
		w.Write(pq.Response.JSON)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h Handler) lockUpload(id string) (tusd.Lock, error) {
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
func (h *Handler) preCreateHook(hook tusd.HookEvent) error {
	r := &http.Request{
		Header: hook.HTTPRequest.Header,
	}
	_, err := h.getUserFromRequest(r)
	return err
}

func (h *Handler) getUserFromRequest(r *http.Request) (*models.User, error) {
	return h.options.userGetter.GetFromRequest(r)
}

func getCaller(sdkAddress, filename string, userID int, qCache *cache.Cache) *query.Caller {
	c := query.NewCaller(sdkAddress, userID)
	c.Cache = qCache
	c.AddPreflightHook(query.AllMethodsHook, func(_ *query.Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
		q := query.GetQuery(ctx)
		params := q.ParamsAsMap()
		params[fileNameParam] = filename
		q.Request.Params = params
		return nil, nil
	}, "")
	return c
}

// observeFailure requires metrics.MeasureMiddleware middleware to be present on the request
func observeFailure(d float64, kind string) {
	metrics.ProxyE2ECallDurations.WithLabelValues(method).Observe(d)
	metrics.ProxyE2ECallFailedDurations.WithLabelValues(method, kind).Observe(d)
	metrics.ProxyE2ECallCounter.WithLabelValues(method).Inc()
	metrics.ProxyE2ECallFailedCounter.WithLabelValues(method, kind).Inc()
}

// observeSuccess requires metrics.MeasureMiddleware middleware to be present on the request
func observeSuccess(d float64) {
	metrics.ProxyE2ECallDurations.WithLabelValues(method).Observe(d)
	metrics.ProxyE2ECallCounter.WithLabelValues(method).Inc()
}
