package geopublish

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/forklift"
	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/app/proxy"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/query/cache"
	"github.com/OdyseeTeam/odysee-api/app/rpcerrors"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/gorilla/mux"
	werrors "github.com/pkg/errors"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/volatiletech/sqlboiler/boil"
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
	FromRequest(*http.Request) (*models.User, error)
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
	logger     logging.KVLogger
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

func WithLogger(logger logging.KVLogger) func(options *HandlerOptions) {
	return func(options *HandlerOptions) {
		options.logger = logger
	}
}

func WithTusConfig(config tusd.Config) func(options *HandlerOptions) {
	return func(options *HandlerOptions) {
		options.tusConfig = &config
	}
}

// NewHandler creates a new geopublish HTTP handler.
func NewHandler(optionFuncs ...func(*HandlerOptions)) (*Handler, error) {
	options := &HandlerOptions{
		// Logger: logging.NoopKVLogger{},
		uploadPath: "./uploads",
		tusConfig:  &tusd.Config{},
		logger:     logging.NoopKVLogger{},
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
	// Allow client to set location response protocol
	// via X-Forwarded-Proto
	cfg.RespectForwardedHeaders = true

	udb := UploadsDB{logger: h.options.logger, db: options.db, queue: options.queue}
	udb.listenToHandler(h)
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
	l := h.options.logger.With("method_handler", "Notify")

	user, err := h.getUserFromRequest(r)
	if authErr := proxy.GetAuthError(user, err); authErr != nil {
		l.Warn("user auth failed", "err", authErr)
		metrics.Errors.WithLabelValues("auth").Inc()
		rpcerrors.Write(w, authErr)
		return
	}
	l = l.With("user_id", user.ID)

	if sdkrouter.GetSDKAddress(user) == "" {
		l.Warn("no sdk assigned")
		metrics.Errors.WithLabelValues("sdk_address").Inc()
		rpcerrors.Write(w, errors.Err("user does not have sdk assigned"))
		return
	}

	params := mux.Vars(r)
	id := params["id"]
	if id == "" {
		err := fmt.Errorf("upload id is required")
		l.Warn("param parse error", "err", err)
		metrics.Errors.WithLabelValues("missing_param").Inc()
		rpcerrors.Write(w, rpcerrors.NewInvalidParamsError(err))
		return
	}

	l = l.With("upload_id", id)

	lock, err := h.lockUpload(id)
	if err != nil {
		monitor.ErrorToSentry(err, map[string]string{
			"upload_id": id,
			"user_id":   strconv.Itoa(user.ID),
		})
		l.Warn("failed to acquire file lock", "err", err)
		metrics.Errors.WithLabelValues("lock").Inc()
		rpcerrors.Write(w, err)
		return
	}
	defer lock.Unlock()

	upload, err := h.composer.Core.GetUpload(r.Context(), id)
	if err != nil {
		l.Warn("failed to get upload object", "err", err)
		metrics.Errors.WithLabelValues("object_load").Inc()
		rpcerrors.Write(w, err)
		return
	}

	info, err := upload.GetInfo(r.Context())
	if err != nil {
		l.Warn("failed to get upload info", "err", err)
		metrics.Errors.WithLabelValues("upload_info").Inc()
		rpcerrors.Write(w, err)
		return
	}

	// NOTE: don't use info.IsFinal as it's not reflect the upload
	// completion at all
	if info.Offset != info.Size { // upload is not yet completed
		err := fmt.Errorf("cannot notify, upload is not finished")
		l.Warn("unfinished upload notify")
		metrics.Errors.WithLabelValues("upload_unfinished").Inc()
		rpcerrors.Write(w, err)
		return
	}

	// uploadMD holds uploaded file metadata sent by client when it first
	// start the upload sequence.
	uploadMD := info.MetaData
	if len(uploadMD) == 0 {
		err := fmt.Errorf("missing file metadata")
		l.Warn("missing file metadata")
		metrics.Errors.WithLabelValues("bad_input_param").Inc()
		w.Write(rpcerrors.ErrorToJSON(err))
		return
	}

	origUploadName, ok := uploadMD["filename"]
	if !ok || origUploadName == "" {
		err := fmt.Errorf("missing file name")
		l.Warn("missing file name")
		metrics.Errors.WithLabelValues("bad_input_param").Inc()
		w.Write(rpcerrors.ErrorToJSON(err))
		return

	}

	origUploadPath, ok := info.Storage["Path"]
	if !ok || origUploadPath == "" { // shouldn't happen but check regardless
		err := fmt.Errorf("missing file path")
		l.Error("storage error", "err", err, "info", reflect.ValueOf(info.Storage).MapKeys())
		metrics.Errors.WithLabelValues("upload_meta").Inc()
		rpcerrors.Write(w, err)
		return
	}

	// rename the uploaded file to the new location
	// with name based on the value from upload metadata.
	dir := filepath.Dir(origUploadPath)

	dstDir := filepath.Join(dir, strconv.Itoa(user.ID), info.ID)
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		l.Error("failed to create directory", "err", err, "path", dstDir)
		metrics.Errors.WithLabelValues("storage").Inc()
		rpcerrors.Write(w, err)
		return
	}

	dstFilepath := filepath.Join(dstDir, origUploadName)
	if err := os.Rename(origUploadPath, dstFilepath); err != nil {
		l.Error("failed to rename file", "err", err, "path", dstDir, "dst_path", dstFilepath)
		metrics.Errors.WithLabelValues("storage").Inc()
		rpcerrors.Write(w, err)
		return
	}

	var rpcReq *jsonrpc.RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpcReq); err != nil {
		l.Error("bad json input received", "err", err)
		metrics.Errors.WithLabelValues("bad_input_json").Inc()
		rpcerrors.Write(w, rpcerrors.NewJSONParseError(err))
		return
	}

	rpcparams, ok := rpcReq.Params.(map[string]interface{})
	if !ok {
		l.Error("bad parameters received")
		metrics.Errors.WithLabelValues("bad_input_json").Inc()
		rpcerrors.Write(w, rpcerrors.NewInvalidParamsError(werrors.New("cannot parse params")))
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
		metrics.Errors.WithLabelValues("storage").Inc()
		l.Error("upload cleanup failed", err, "path", infoFile)
		monitor.ErrorToSentry(err, map[string]string{"info_file": infoFile})
	}

	ctx := logging.AddToContext(context.Background(), l)
	err = h.udb.startProcessingUpload(ctx, info.ID, user, dstFilepath, rpcReq)
	if err != nil {
		l.Error("upload processing failed", "err", err)
		rpcerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// Status returns status of the upload.
// Possible response HTTP codes:
// - 202: upload is currently being processed
// - 200: upload has been fully processed and is immediately available on the network. Normal JSON-RPC SDK response is provided in the body
// - 409: SDK returned an error and upload cannot be processed. Error details are provided in the response body
// - 404, 403: upload not found or does not belong to the user
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
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			rpcerrors.Write(w, err)
		}
		return
	}

	pq, err := up.Query().One(h.udb.db)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		rpcerrors.Write(w, fmt.Errorf("error getting publish query: %w", err))
		return
	}

	switch pq.Status {
	case models.QueryStatusSucceeded:
		w.WriteHeader(http.StatusOK)
		w.Write(pq.Response.JSON)
	case models.QueryStatusFailed:
		if pq.Response.IsZero() {
			w.Write(pq.Response.JSON)
		} else {
			rpcerrors.Write(w, errors.Err(pq.Error))
		}
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
	return h.options.userGetter.FromRequest(r)
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
