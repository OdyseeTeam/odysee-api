package uploads

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/uploads/database"
	"github.com/OdyseeTeam/odysee-api/internal/tasks"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/queue"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/hibiken/asynq"
	"github.com/lestrrat-go/jwx/v2/jwt"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/tus/tusd/pkg/prometheuscollector"
	"github.com/tus/tusd/pkg/s3store"
)

const (
	AuthorizationHeader = "Authorization"
	userContextKey      = "user"
)

var (
	onceMetrics sync.Once

	reExtractFileID = regexp.MustCompile(`([^/]{32,})\/?$`)
)

var TusHeaders = []string{
	"Http-Method-Override",
	"Upload-Length",
	"Upload-Offset",
	"Tus-Resumable",
	"Upload-Metadata",
	"Upload-Defer-Length",
	"Upload-Concat",
}

// Handler handle media publishing on odysee-api, it implements TUS
// specifications to support resumable file upload and extends the handler to
// support fetching media from remote url.
type Handler struct {
	*tusd.UnroutedHandler
	s3bucket       string
	queries        *database.Queries
	logger         logging.KVLogger
	jwtAuth        *jwtauth.JWTAuth
	tokenValidator *keybox.Validator
	notifier       *forkliftNotifier
	stopChan       chan struct{}
}

type LauncherOption func(*Launcher)

type Launcher struct {
	corsDomains   []string
	db            database.DBTX
	fileLocker    tusd.Locker
	httpAddress   string
	prefix        string
	publicKey     crypto.PublicKey
	router        chi.Router
	queueRedisURL string
	s3client      *s3.S3
	s3bucket      string
	handler       *Handler
	httpServer    *http.Server
	logger        logging.KVLogger
	notifier      *forkliftNotifier
	readyCancel   context.CancelFunc
}

type forkliftNotifier struct {
	queries *database.Queries
	queue   *queue.Queue
	logger  logging.KVLogger
}

func WithLogger(logger logging.KVLogger) LauncherOption {
	return func(l *Launcher) {
		l.logger = logger
	}
}

func WithS3Client(client *s3.S3) LauncherOption {
	return func(l *Launcher) {
		l.s3client = client
	}
}

func WithS3Bucket(bucket string) LauncherOption {
	return func(l *Launcher) {
		l.s3bucket = bucket
	}
}

func WithFileLocker(fileLocker tusd.Locker) LauncherOption {
	return func(l *Launcher) {
		l.fileLocker = fileLocker
	}
}

func WithPrefix(prefix string) LauncherOption {
	return func(l *Launcher) {
		l.prefix = prefix
	}
}

func WithPublicKey(publicKey crypto.PublicKey) LauncherOption {
	return func(l *Launcher) {
		l.publicKey = publicKey
	}
}

func WithForkliftRequestsConnURL(url string) LauncherOption {
	return func(l *Launcher) {
		l.queueRedisURL = url
	}
}

func WithHTTPAddress(address string) LauncherOption {
	return func(l *Launcher) {
		l.httpAddress = address
	}
}

func WithDB(db database.DBTX) LauncherOption {
	return func(l *Launcher) {
		l.db = db
	}
}

func WithCORSDomains(domains []string) LauncherOption {
	return func(l *Launcher) {
		l.corsDomains = domains
	}
}

func NewLauncher(options ...LauncherOption) *Launcher {
	launcher := &Launcher{
		logger:      logging.NoopKVLogger{},
		prefix:      "/v1",
		httpAddress: "0.0.0.0:8080",
		corsDomains: []string{""},
	}

	for _, opt := range options {
		opt(launcher)
	}

	return launcher
}

func (l *Launcher) Notifier() (*forkliftNotifier, error) {
	if l.queueRedisURL == "" {
		l.logger.Warn("skipping bus config as no redis url was provided")
		return nil, nil
	}
	opts, err := asynq.ParseRedisURI(l.queueRedisURL)
	if err != nil {
		return nil, err
	}
	queue, err := queue.New(queue.WithRequestsConnOpts(opts), queue.WithLogger(l.logger))
	if err != nil {
		return nil, err
	}
	notifier := &forkliftNotifier{
		queries: database.New(l.db),
		queue:   queue,
		logger:  l.logger,
	}
	l.logger.Info("bus client created")

	return notifier, nil
}

func (l *Launcher) BuildHandler() (chi.Router, error) {
	validator, err := keybox.NewValidator(l.publicKey)
	if err != nil {
		return nil, err
	}

	l.logger.Info("creating s3 bucket", "bucket", l.s3bucket)
	_, err = l.s3client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(l.s3bucket),
	})
	if err != nil {
		var awsError awserr.Error
		if errors.As(err, &awsError) && awsError.Code() == "BucketAlreadyOwnedByYou" {
			l.logger.Info("bucket already exists", "bucket", l.s3bucket)
		} else {
			return nil, err
		}
	}

	l.logger.Info("building uploads handler")
	notifier, err := l.Notifier()
	if err != nil {
		return nil, err
	}

	readyCtx, readyCancel := context.WithCancel(context.Background())
	store := s3store.New(l.s3bucket, l.s3client)
	handler := &Handler{
		s3bucket:       l.s3bucket,
		logger:         l.logger,
		queries:        database.New(l.db),
		tokenValidator: validator,
		notifier:       notifier,
		stopChan:       make(chan struct{}),
	}
	l.readyCancel = readyCancel

	composer := tusd.NewStoreComposer()
	composer.UseLocker(l.fileLocker)
	store.UseIn(composer)

	tusConfig := tusd.Config{
		StoreComposer:           composer,
		BasePath:                l.prefix + "/uploads",
		RespectForwardedHeaders: true,
		NotifyCreatedUploads:    true,
		NotifyTerminatedUploads: true,
		NotifyUploadProgress:    true,
		NotifyCompleteUploads:   true,
	}

	httpLogger := &JSONLogger{logger: l.logger}

	if iol, ok := l.logger.(io.Writer); ok {
		l.logger.Info("attaching logger to tusd")
		tusConfig.Logger = stdlog.New(iol, "", 0)
	}

	tusdHandler, err := tusd.NewUnroutedHandler(tusConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create tusd handler: %v", err)
	}
	handler.UnroutedHandler = tusdHandler

	router := chi.NewRouter()
	router.Use(httpLogger.Middleware)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   l.corsDomains,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodHead, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   append([]string{"Accept", "Authorization", "Content-Type", "X-Requested-With"}, TusHeaders...),
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	registry := prometheus.NewRegistry()
	onceMetrics.Do(func() {
		registerMetrics(registry)
		redislocker.RegisterMetrics(registry)
		tusMetrics := prometheuscollector.New(handler.Metrics)
		registry.MustRegister(tusMetrics)
	})
	promHandler := promhttp.InstrumentMetricHandler(
		registry, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	)

	// Public endpoints
	router.Route(l.prefix, func(r chi.Router) {
		r.Use(handler.authByToken)
		r.Route("/uploads", func(r chi.Router) {
			r.Use(handler.Middleware)
			r.Use(handler.verifyUploadFileAccess)
			r.Post("/", handler.PostFile)
			r.Head("/*", handler.HeadFile)
			r.Patch("/*", handler.PatchFile)
			r.Delete("/*", handler.DelFile)
		})
		r.Route("/urls", func(r chi.Router) {
			r.Post("/", handler.PostURL)
		})
	})

	// Internal endpoints
	router.Get("/livez", func(w http.ResponseWriter, _ *http.Request) {
		if readyCtx.Err() != nil {
			http.Error(w, "upload service is shutting down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("OK"))
	})
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("OK"))
	})

	router.Get("/internal/metrics", promHandler.ServeHTTP)

	httpServer := &http.Server{
		Addr:    l.httpAddress,
		Handler: router,
	}

	l.router = router
	l.httpServer = httpServer
	l.handler = handler
	l.logger.Info("uploads handler built")
	handler.listenToHooks()
	return router, nil
}

func (l *Launcher) Launch() {
	l.logger.Info("launching http server", "address", l.httpAddress)
	err := l.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		l.logger.Error("http server returned error", "err", err)
	}
	l.logger.Info("http server stopped")
}

func (l *Launcher) CompleteShutdown() {
	l.logger.Info("shutting down http server")
	err := l.httpServer.Shutdown(context.Background())
	if err != nil {
		l.logger.Info("error encountered while stopping http server", "err", err)
	}
	l.logger.Info("shutting down upload listeners")
	close(l.handler.stopChan)
}

func (l *Launcher) StartShutdown() {
	l.logger.Info("shutting down liveness handler")
	l.readyCancel()
}

func (h *Handler) PostURL(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserIDFromRequest(r)
	data := &URLPayload{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}
	_, err := h.queries.CreateURL(r.Context(), database.CreateURLParams{
		UserID:   userID,
		ID:       data.UploadID,
		URL:      data.URL,
		Filename: data.Filename,
	})

	if err != nil {
		render.Render(w, r, ErrInternalError(err))
		return
	}
	err = h.notifier.URLReceived(userID, data.UploadID, data.Filename, tasks.FileLocationHTTP{URL: data.URL})
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}
	render.Render(w, r, ResponseURLCreated(data.UploadID))
}

func (h *Handler) listenToHooks() {
	h.logger.Info("listening to upload signals")

	listen := func(ch chan tusd.HookEvent, eventHandler func(uid int32, event tusd.HookEvent)) {
		h.logger.Info("starting signal listener")
		defer func() { h.logger.Info("stopping signal listener") }()
		for {
			select {
			case event := <-ch:
				userID, err := h.extractUserIDFromRequest(&http.Request{Header: event.HTTPRequest.Header})
				if err != nil {
					h.logger.Error("failed to extract user id from tus event", "err", err, "headers", event.HTTPRequest.Header.Get(AuthorizationHeader))
				}
				eventHandler(userID, event)
			case <-h.stopChan:
				return
			}
		}
	}

	go listen(h.CreatedUploads, func(uid int32, event tusd.HookEvent) {
		p := database.CreateUploadParams{
			UserID: uid,
			ID:     event.Upload.ID,
			Size:   event.Upload.Size,
		}
		_, err := h.queries.CreateUpload(context.Background(), p)
		if err != nil {
			sqlErrors.Inc()
			h.logger.Warn("creating upload failed", "user_id", uid, "upload_id", p.ID, "err", err)
			return
		}
		h.logger.Info("upload created", "user_id", uid, "upload_id", p.ID)
	})

	go listen(h.UploadProgress, func(uid int32, event tusd.HookEvent) {
		p := database.RecordUploadProgressParams{
			UserID:   uid,
			ID:       event.Upload.ID,
			Received: event.Upload.Offset,
		}
		err := h.queries.RecordUploadProgress(context.Background(), p)
		if err != nil {
			sqlErrors.Inc()
			h.logger.Warn("recording upload progress failed", "user_id", p.UserID, "upload_id", p.ID, "err", err)
			return
		}
		h.logger.Debug("upload progress", "user_id", uid, "upload_id", p.ID, "size", event.Upload.Size, "received", p.Received)
	})

	go listen(h.TerminatedUploads, func(uid int32, event tusd.HookEvent) {
		p := database.MarkUploadTerminatedParams{
			UserID: uid,
			ID:     event.Upload.ID,
		}
		err := h.queries.MarkUploadTerminated(context.Background(), p)
		if err != nil {
			sqlErrors.Inc()
			h.logger.Warn("recording upload termination failed", "user_id", uid, "upload_id", p.ID, "err", err)
			return
		}
		h.logger.Info("upload terminated", "user_id", uid, "upload_id", p.ID)
	})

	go listen(h.CompleteUploads, func(uid int32, event tusd.HookEvent) {
		p := database.MarkUploadCompletedParams{
			UserID:   uid,
			ID:       event.Upload.ID,
			Filename: event.Upload.MetaData["filename"],
			Key:      event.Upload.Storage["Key"],
		}
		err := h.queries.MarkUploadCompleted(context.Background(), p)
		if err != nil {
			sqlErrors.Inc()
			h.logger.Warn("recording upload completion failed", "user_id", uid, "upload_id", p.ID, "err", err)
			return
		}

		if h.notifier == nil {
			return
		}
		err = h.notifier.UploadReceived(
			uid,
			event.Upload.ID,
			event.Upload.MetaData["filename"],
			tasks.FileLocationS3{
				Key:    p.Key,
				Bucket: h.s3bucket,
			})
		if err != nil {
			h.logger.Warn("completing upload failed", "user_id", uid, "upload_id", event.Upload.ID, "err", err)
			redisErrors.Inc()
			return
		}
		h.logger.Info("upload received", "user_id", uid, "upload_id", event.Upload.ID, "size", event.Upload.Size)
	})
	h.stopChan = make(chan struct{})
}

// authByToken is the primary authentication middleware that accepts tokens generated by asynquery create upload call.
func (h *Handler) authByToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		userID, err := h.extractUserIDFromRequest(r)
		if err != nil {
			h.logger.Info("failed to extract token from request", "err", err)
			userAuthErrors.Inc()
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), userContextKey, userID)))
	})
}

// verifyUploadFileAccess is a middleware to verify that accessed upload is owned by the user.
func (h *Handler) verifyUploadFileAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadID := extractUploadIDFromPath(r.URL.Path)
		userID := h.getUserIDFromRequest(r)
		if userID == 0 {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		if uploadID != "" {
			_, err := h.queries.GetUpload(context.TODO(), database.GetUploadParams{
				UserID: userID, ID: uploadID,
			})
			if err != nil {
				h.logger.Info("upload not found", "upload_id", uploadID, "user_id", userID, "err", err)
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
		}

		next.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), userContextKey, userID)))
	})
}

func (h *Handler) getUserIDFromRequest(r *http.Request) int32 {
	return r.Context().Value(userContextKey).(int32)
}

// extractUserIDFromRequest retrieves token from request header and extracts user ID from it
func (h *Handler) extractUserIDFromRequest(r *http.Request) (int32, error) {
	rt := jwtauth.TokenFromHeader(r)
	if rt == "" {
		return 0, errors.New("missing authentication token in request")
	}
	token, err := h.tokenValidator.ParseToken(rt)
	if err != nil {
		return 0, err
	}

	if err := jwt.Validate(token); err != nil {
		return 0, fmt.Errorf("cannot validate token: %w", err)
	}
	if token.Subject() == "" {
		return 0, errors.New("missing user id in token")
	}
	uid, err := strconv.ParseInt(token.Subject(), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("cannot parse user id: %w", err)
	}
	return int32(uid), nil
}

// extractUploadIDFromPath pulls the last segment from the url provided
func extractUploadIDFromPath(url string) string {
	result := reExtractFileID.FindStringSubmatch(url)
	if len(result) != 2 {
		return ""
	}
	return result[1]
}

// UploadReceived sends off a finalized upload to forklift queue for further processing.
func (c forkliftNotifier) UploadReceived(userID int32, uploadID, filename string, location tasks.FileLocationS3) error {
	err := c.queue.SendRequest(
		tasks.ForkliftUploadIncoming,
		tasks.ForkliftUploadIncomingPayload{
			UploadID:     uploadID,
			UserID:       userID,
			FileName:     filename,
			FileLocation: location,
		}, queue.WithRequestRetry(10), queue.WithRequestTimeout(24*time.Hour),
	)
	if err != nil {
		return err
	}
	c.logger.Info("forklift notified", "type", "upload", "upload_id", uploadID, "user_id", userID)
	return nil
}

// URLReceived sends off a finalized upload to forklift queue for further processing.
func (c forkliftNotifier) URLReceived(userID int32, uploadID, filename string, location tasks.FileLocationHTTP) error {
	err := c.queue.SendRequest(
		tasks.ForkliftURLIncoming,
		tasks.ForkliftURLIncomingPayload{
			UserID:       userID,
			UploadID:     uploadID,
			FileName:     filename,
			FileLocation: location,
		}, queue.WithRequestRetry(10), queue.WithRequestTimeout(24*time.Hour),
	)
	if err != nil {
		return err
	}
	c.logger.Info("forklift notified", "type", "url", "url", location.URL, "user_id", userID)
	return nil
}
