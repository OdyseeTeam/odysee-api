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
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/uploads/database"
	"github.com/OdyseeTeam/odysee-api/internal/tasks"
	"github.com/OdyseeTeam/odysee-api/pkg/bus"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/hibiken/asynq"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/tus/tusd/pkg/s3store"
)

const (
	AuthorizationHeader = "Authorization"
)

var (
	reExtractFileID = regexp.MustCompile(`([^/]{32,})\/?$`)
)

type Launcher struct {
	corsDomains []string
	db          database.DBTX
	fileLocker  tusd.Locker
	httpAddress string
	prefix      string
	publicKey   crypto.PublicKey
	router      chi.Router
	busRedisURL string
	s3Store     s3store.S3Store
	handler     *Handler
	httpServer  *http.Server
	logger      logging.KVLogger
	readyCancel context.CancelFunc
}

// Handler handle media publishing on odysee-api, it implements TUS
// specifications to support resumable file upload and extends the handler to
// support fetching media from remote url.
type Handler struct {
	*tusd.UnroutedHandler
	bus            *bus.Client
	bucketName     string
	queries        *database.Queries
	logger         logging.KVLogger
	jwtAuth        *jwtauth.JWTAuth
	tokenValidator *keybox.Validator
	stopChan       chan struct{}
}

func NewLauncher() *Launcher {
	launcher := &Launcher{
		logger:      logging.NoopKVLogger{},
		prefix:      "/v1/uploads",
		httpAddress: ":8080",
	}
	return launcher
}

func (c *Launcher) Logger(logger logging.KVLogger) *Launcher {
	c.logger = logger
	return c
}

func (c *Launcher) Store(s s3store.S3Store) *Launcher {
	c.s3Store = s
	return c
}

func (c *Launcher) FileLocker(fileLocker tusd.Locker) *Launcher {
	c.fileLocker = fileLocker
	return c
}

func (c *Launcher) Prefix(prefix string) *Launcher {
	c.prefix = prefix
	return c
}

func (c *Launcher) PublicKey(publicKey crypto.PublicKey) *Launcher {
	c.publicKey = publicKey
	return c
}

func (c *Launcher) BusRedisURL(busRedisURL string) *Launcher {
	c.busRedisURL = busRedisURL
	return c
}

func (c *Launcher) HTTPAddress(address string) *Launcher {
	c.httpAddress = address
	return c
}

func (c *Launcher) DB(db database.DBTX) *Launcher {
	c.db = db
	return c
}

func (c *Launcher) CORSDomains(domains []string) *Launcher {
	c.corsDomains = domains
	return c
}

func (l *Launcher) Build() (chi.Router, error) {
	validator, err := keybox.NewValidator(l.publicKey)
	if err != nil {
		return nil, err
	}

	l.logger.Info("building upload handler")
	readyCtx, readyCancel := context.WithCancel(context.Background())
	handler := &Handler{
		bucketName:     l.s3Store.Bucket,
		logger:         l.logger,
		queries:        database.New(l.db),
		tokenValidator: validator,
	}
	l.readyCancel = readyCancel

	if l.busRedisURL != "" {
		redisOpts, err := asynq.ParseRedisURI(l.busRedisURL)
		if err != nil {
			return nil, err
		}
		bus, err := bus.NewClient(redisOpts, l.logger)
		if err != nil {
			return nil, err
		}
		handler.bus = bus
		l.logger.Info("bus client created")
	}

	composer := tusd.NewStoreComposer()
	composer.UseLocker(l.fileLocker)
	l.s3Store.UseIn(composer)

	tusConfig := tusd.Config{
		StoreComposer:           composer,
		BasePath:                l.prefix,
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
		return nil, fmt.Errorf("Unable to create tusd handler: %v", err)
	}
	handler.UnroutedHandler = tusdHandler

	router := chi.NewRouter()
	router.Use(httpLogger.Middleware)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   l.corsDomains,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodHead, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	router.Get("/livez", func(w http.ResponseWriter, r *http.Request) {
		if readyCtx.Err() != nil {
			http.Error(w, "upload service is shutting down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("OK"))
	})
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})
	router.Route(l.prefix, func(r chi.Router) {
		r.Use(handler.authByToken)
		r.Use(handler.Middleware)
		r.Post("/", handler.PostFile)
		r.Head("/*", handler.HeadFile)
		r.Patch("/*", handler.PatchFile)
		r.Delete("/*", handler.DelFile)
	})

	httpServer := &http.Server{
		Addr:    l.httpAddress,
		Handler: l.router,
	}

	l.router = router
	l.httpServer = httpServer
	handler.listenToHooks()
	return router, nil
}

func (l *Launcher) ServerShutdown() {
	err := l.httpServer.Shutdown(context.Background())
	if err != nil {
		l.logger.Info("error encountered while stopping http server", "err", err)
	}
}

func (l *Launcher) StartShutdown() {
	l.logger.Info("shutting down liveness handler")
	l.readyCancel()
}

func (l *Launcher) CompleteShutdown() {
	l.logger.Info("shutting down upload listeners")
	close(l.handler.stopChan)
}

func (l *Launcher) Launch() {
	err := l.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		l.logger.Error("http server returned error", "err", err)
	}
	l.logger.Info("http server stopped")
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
			h.logger.Warn("recording upload termination failed", "user_id", uid, "upload_id", p.ID, "err", err)
			return
		}
		h.logger.Info("upload terminated", "user_id", uid, "upload_id", p.ID)
	})

	go listen(h.CompleteUploads, func(uid int32, event tusd.HookEvent) {
		err := completeUpload(
			h.queries, h.bus, uid, event.Upload.ID, event.Upload.MetaData["filename"], h.bucketName, event.Upload.Storage["Key"])
		if err != nil {
			h.logger.Warn("completing upload failed", "user_id", uid, "upload_id", event.Upload.ID, "err", err)
			return
		}
		h.logger.Info("upload completed", "user_id", uid, "upload_id", event.Upload.ID, "size", event.Upload.Size)
	})
	h.stopChan = make(chan struct{})
}

// authByToken is the primary authentication middleware to enforce access to upload URLs for a given user.
// authByToken sends a 401 Unauthorized response for unverified tokens and 404 if no matching upload found for user.
func (h *Handler) authByToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := h.extractUserIDFromRequest(r)

		if err != nil {
			h.logger.Info("failed to extract token from request", "err", err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		upid := extractUploadIDFromPath(r.URL.Path)
		if upid != "" {
			_, err := h.queries.GetUpload(context.TODO(), database.GetUploadParams{
				UserID: userID, ID: upid,
			})
			if err != nil {
				h.logger.Info("upload not found", "upload_id", upid, "user_id", userID, "err", err)
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) extractUserIDFromRequest(r *http.Request) (int32, error) {
	rt := jwtauth.TokenFromHeader(r)
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

func completeUpload(queries *database.Queries, b *bus.Client, uid int32, uploadID, fileName, bucket, key string) error {
	p := database.MarkUploadCompletedParams{
		UserID:   uid,
		ID:       uploadID,
		Filename: fileName,
		Key:      key,
	}
	err := queries.MarkUploadCompleted(context.Background(), p)
	if err != nil {
		return err
	}

	if b != nil {
		if err != nil {
			return err
		}
		err = b.Put(
			tasks.TaskReflectUpload,
			tasks.ReflectUploadPayload{
				UploadID: p.ID,
				UserID:   uid,
				FileName: p.Filename,
				FileLocation: tasks.FileLocationS3{
					Key:    p.Key,
					Bucket: bucket,
				},
			}, 10, 6*time.Hour, 72*time.Hour,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
