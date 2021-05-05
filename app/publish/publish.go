package publish

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/query"
	"github.com/lbryio/lbrytv/app/query/cache"
	"github.com/lbryio/lbrytv/app/rpcerrors"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/gorilla/mux"
	werrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

var logger = monitor.NewModuleLogger("publish")

var method = "publish"

const (
	MaxRemoteFileSize = 5 << 30 // 4GB

	// fileFieldName refers to the POST field containing file upload
	fileFieldName = "file"
	// jsonRPCFieldName is a name of the POST field containing JSONRPC request accompanying the uploaded file
	jsonRPCFieldName = "json_payload"
	opName           = "publish"

	fileNameParam  = "file_path"
	remoteURLParam = "remote_url"
)

var ErrEmptyRemoteURL = werrors.New("empty remote url")

// Handler has path to save uploads to
type Handler struct {
	UploadPath string
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

// Handle is where HTTP upload is handled and passed on to Publisher.
// It should be wrapped with users.Authenticator.Wrap before it can be used
// in a mux.Router.
func (h Handler) Handle(w http.ResponseWriter, r *http.Request) {
	user, err := auth.FromRequest(r)
	if authErr := proxy.GetAuthError(user, err); authErr != nil {
		w.Write(rpcerrors.ErrorToJSON(authErr))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindAuth)
		return
	}
	if sdkrouter.GetSDKAddress(user) == "" {
		w.Write(rpcerrors.NewInternalError(errors.Err("user does not have sdk address assigned")).JSON())
		logger.Log().Errorf("user %d does not have sdk address assigned", user.ID)
		observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
		return
	}

	log := logger.WithFields(logrus.Fields{"user_id": user.ID, "method_handler": method})

	f, err := h.fetchFile(r, user.ID)
	if err != nil {
		if err == ErrEmptyRemoteURL {
			f, err = h.saveFile(r, user.ID)
			if err != nil {
				log.Error(err)
				monitor.ErrorToSentry(err)
				w.Write(rpcerrors.NewInternalError(err).JSON())
				observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
				return
			}
		} else {
			w.Write(rpcerrors.NewInternalError(err).JSON())
			observeFailure(metrics.GetDuration(r), metrics.FailureKindClient)
			return
		}
	}

	defer func() {
		op := metrics.StartOperation(opName, "remove_file")
		defer op.End()

		if err := os.Remove(f.Name()); err != nil {
			monitor.ErrorToSentry(err, map[string]string{"file_path": f.Name()})
		}
	}()

	var qCache cache.QueryCache
	if cache.IsOnRequest(r) {
		qCache = cache.FromRequest(r)
	}

	var rpcReq *jsonrpc.RPCRequest
	err = json.Unmarshal([]byte(r.FormValue(jsonRPCFieldName)), &rpcReq)
	if err != nil {
		w.Write(rpcerrors.NewJSONParseError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindClientJSON)
		return
	}

	c := getCaller(sdkrouter.GetSDKAddress(user), f.Name(), user.ID, qCache)

	op := metrics.StartOperation("sdk", "call_publish")
	rpcRes, err := c.Call(rpcReq)
	op.End()
	if err != nil {
		monitor.ErrorToSentry(
			fmt.Errorf("error calling publish: %v", err),
			map[string]string{
				"request":  fmt.Sprintf("%+v", rpcReq),
				"response": fmt.Sprintf("%+v", rpcRes),
			},
		)
		logger.Log().Errorf("error calling publish: %v, request: %+v", err, rpcReq)
		w.Write(rpcerrors.ToJSON(err))
		observeFailure(metrics.GetDuration(r), metrics.FailureKindRPC)
		return
	}

	serialized, err := responses.JSONRPCSerialize(rpcRes)
	if err != nil {
		monitor.ErrorToSentry(err)
		logger.Log().Errorf("error marshaling response: %v", err)
		w.Write(rpcerrors.NewInternalError(err).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindRPCJSON)
		return
	}

	w.Write(serialized)
	observeSuccess(metrics.GetDuration(r))
}

func getCaller(sdkAddress, filename string, userID int, qCache cache.QueryCache) *query.Caller {
	c := query.NewCaller(sdkAddress, userID)
	c.Cache = qCache
	c.AddPreflightHook(query.AllMethodsHook, func(_ *query.Caller, hctx *query.HookContext) (*jsonrpc.RPCResponse, error) {
		params := hctx.Query.ParamsAsMap()
		params[fileNameParam] = filename
		hctx.Query.Request.Params = params
		return nil, nil
	}, "")
	return c
}

// CanHandle checks if http.Request contains POSTed data in an accepted format.
// Supposed to be used in gorilla mux router MatcherFunc.
func (h Handler) CanHandle(r *http.Request, _ *mux.RouteMatch) bool {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		return false
	}

	return r.FormValue(jsonRPCFieldName) != ""
}

func (h Handler) saveFile(r *http.Request, userID int) (*os.File, error) {
	op := metrics.StartOperation(opName, "save_file")
	defer op.End()

	log := logger.WithFields(logrus.Fields{"user_id": userID, "method_handler": method})

	file, header, err := r.FormFile(fileFieldName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	f, err := h.createFile(userID, header.Filename)
	if err != nil {
		return nil, err
	}
	log.Infof("processing uploaded file %v", header.Filename)

	numWritten, err := io.Copy(f, file)
	if err != nil {
		return nil, err
	}
	log.Infof("saved uploaded file %v (%v bytes written)", f.Name(), numWritten)

	if err := f.Close(); err != nil {
		return nil, err
	}
	return f, nil
}

// createFile opens an empty file for writing inside the account's designated folder.
// The final file path looks like `/upload_path/{user_id}/{random}_filename.ext`,
// where `user_id` is user's ID and `random` is a random string generated by ioutil.
func (h Handler) createFile(userID int, origFilename string) (*os.File, error) {
	path := path.Join(h.UploadPath, fmt.Sprintf("%d", userID))
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return nil, err
	}
	return ioutil.TempFile(path, fmt.Sprintf(origFilename, "_*%s"))
}

// fetchFile downloads remote file from the URL provided by client.
// ErrEmptyRemoteURL is a standard error when no URL has been provided.
func (h Handler) fetchFile(r *http.Request, userID int) (*os.File, error) {
	log := logger.WithFields(logrus.Fields{"user_id": userID, "method_handler": method})

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		return nil, err
	}

	url := r.Form.Get(remoteURLParam)
	if url == "" {
		return nil, ErrEmptyRemoteURL
	}

	r, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, werrors.Wrap(err, "error creating request")
	}
	fname := path.Base(r.URL.Path)
	if fname == "/" || fname == "." {
		return nil, fmt.Errorf("couldn't determine remote file name")
	}

	timeout := sdkrouter.RPCTimeout - (120 * time.Second)
	c := &http.Client{
		Timeout: timeout,
	}
	resp, err := c.Do(r)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote server returned non-OK status %v", resp.StatusCode)
	}

	defer resp.Body.Close()

	clh := resp.Header.Get("Content-Length")
	cl, err := strconv.Atoi(clh)
	if err != nil {
		return nil, werrors.Wrap(err, "cannot determine remote file size")
	}
	if cl >= MaxRemoteFileSize {
		return nil, fmt.Errorf("remote file is too large at %v bytes", cl)
	}
	if cl == 0 {
		return nil, werrors.New("remote file is empty")
	}

	f, err := h.createFile(userID, fname)
	if err != nil {
		return nil, err
	}
	log.Infof("processing remote file %v", fname)

	numWritten, err := io.Copy(f, resp.Body)
	if err != nil {
		return nil, err
	}
	if numWritten == 0 {
		f.Close()
		os.Remove(f.Name())
		return f, werrors.New("remote file is empty")
	}
	log.Infof("saved uploaded file %v (%v bytes written)", f.Name(), numWritten)

	if err := f.Close(); err != nil {
		return f, err
	}

	return f, nil
}
