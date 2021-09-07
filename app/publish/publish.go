package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-retryablehttp"
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
	FetchSizeLimit = 6000000000

	// fileFieldName refers to the POST field containing file upload
	fileFieldName = "file"
	// jsonRPCFieldName is a name of the POST field containing JSONRPC request accompanying the uploaded file
	jsonRPCFieldName = "json_payload"
	opName           = "publish"
	fileNameParam    = "file_path"
	remoteURLParam   = "remote_url"
)

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

// CanHandle checks if http.Request contains POSTed data in an accepted format.
// Supposed to be used in gorilla mux router MatcherFunc.
func CanHandle(r *http.Request, _ *mux.RouteMatch) bool {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		return false
	}
	return r.FormValue(jsonRPCFieldName) != ""
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

	tries := 1

retry:
	log = log.WithField("tries", tries)
	ctx, cancel := context.WithTimeout(r.Context(), fetchTimeout)
	defer cancel()

	f, err := h.fetchFile(r, user.ID)
	if err != nil {
		switch err.(type) {
		case *RequestError:
			w.Write(rpcerrors.NewInternalError(err).JSON())
			observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
			return

		case *FetchError:
			if errors.Is(err, ErrEmptyRemoteURL) {
				f, err = h.saveFile(r, user.ID)
				if err != nil {
					log.Error(err)
					monitor.ErrorToSentry(err)
					w.Write(rpcerrors.NewInternalError(err).JSON())
					observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
					return
				}
			} else {
				tries++
				if tries > fetchTryLimit {
					log.WithError(err).Warn("failed to fetch remote file")
					w.Write(rpcerrors.NewInternalError(err).JSON())
					observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
					return
				}

				log.WithError(err).Warnf("retrying fetch remote file")
				time.Sleep(fetchRetryDelay)

				goto retry
			}

		default:
			log.WithError(err).Warn("unexpected error")
			w.Write(rpcerrors.NewInternalError(err).JSON())
			observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
			return
		}
	}

	select {
	case <-ctx.Done():
		log.WithError(ctx.Err()).Error("hitting timeout while retrying fetch remote file")
		w.Write(rpcerrors.NewInternalError(ctx.Err()).JSON())
		observeFailure(metrics.GetDuration(r), metrics.FailureKindInternal)
		return

	default:
		// all good, continue
	}

	defer func() {
		op := metrics.StartOperation(opName, "remove_file")
		defer op.End()

		if err := os.RemoveAll(filepath.Dir(f.Name())); err != nil {
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
// The final file path looks like `/upload_path/{user_id}/{random}/filename.ext
// where `user_id` is user's ID and `random` is a random string generated by ioutil.
func (h Handler) createFile(userID int, origFilename string) (*os.File, error) {
	uploadPath := path.Join(h.UploadPath, fmt.Sprintf("%d", userID))
	err := os.MkdirAll(uploadPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	randomDir, err := os.MkdirTemp(uploadPath, "")
	if err != nil {
		return nil, err
	}

	return os.Create(path.Join(randomDir, origFilename))
}

// fetchFile downloads remote file from the URL provided by client.
// ErrEmptyRemoteURL is a standard error when no URL has been provided.
func (h Handler) fetchFile(r *http.Request, userID int) (*os.File, error) {
	log := logger.WithFields(logrus.Fields{"user_id": userID, "method_handler": method})

	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB
		return nil, &RequestError{Err: err, Msg: "invalid request payload"}
	}

	urlstring := r.Form.Get(remoteURLParam)
	if urlstring == "" {
		return nil, ErrEmptyRemoteURL
	}

	c := &retryablehttp.Client{
		HTTPClient: &http.Client{
			Transport: cleanhttp.DefaultPooledTransport(),
			Timeout:   defaultRequestTimeout,
		},
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
		Backoff:      retryablehttp.DefaultBackoff,
		CheckRetry:   retryPolicy,
		ErrorHandler: retryablehttp.PassthroughErrorHandler,
	}

	httpReq, err := http.NewRequest(
		http.MethodGet,
		urlstring,
		http.NoBody,
	)
	if err != nil {
		return nil, &FetchError{urlstring, werrors.Wrap(err, "error creating request")}
	}

	fname := path.Base(httpReq.URL.Path)
	if fname == "/" || fname == "." || fname == "" {
		return nil, &FetchError{urlstring, fmt.Errorf("couldn't determine remote file name")}
	}

	resp, err := c.Do(&retryablehttp.Request{
		Request: httpReq,
	})
	if err != nil {
		return nil, &FetchError{urlstring, err}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &FetchError{urlstring, fmt.Errorf("remote server returned non-OK status %v", resp.StatusCode)}
	}

	defer resp.Body.Close()

	f, err := h.createFile(userID, fname)
	if err != nil {
		return nil, &FetchError{urlstring, err}
	}
	log.Infof("processing remote file %v", fname)

	numWritten, err := io.Copy(f, resp.Body)
	if err != nil {
		return nil, &FetchError{urlstring, err}
	}
	if numWritten == 0 {
		f.Close()
		os.Remove(f.Name())

		return f, &FetchError{urlstring, werrors.New("remote file is empty")}
	}
	log.Infof("saved uploaded file %v (%v bytes written)", f.Name(), numWritten)

	if err := f.Close(); err != nil {
		return f, &FetchError{urlstring, err}
	}

	return f, nil
}
