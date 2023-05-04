package upload

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"regexp"

	"github.com/OdyseeTeam/odysee-api/apps/upload/database"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/tus/tusd/pkg/s3store"
)

const (
	AuthorizationHeader = "Authorization"

	jwtSecret = "your-jwt-secret"
	s3Bucket  = "your-s3-bucket"
)

var (
	reExtractUserID = regexp.MustCompile(`^/[\w/]+/(\d+)/`)
	reExtractFileID = regexp.MustCompile(`([^/]{32,})\/?$`)
)

type Launcher struct {
	prefix      string
	logger      logging.KVLogger
	s3Store     s3store.S3Store
	fileLocker  tusd.Locker
	publicKey   crypto.PublicKey
	httpAddress string
	router      chi.Router
	db          database.DBTX
	handler     *Handler
	httpServer  *http.Server
	readyCancel context.CancelFunc
}

// Handler handle media publishing on odysee-api, it implements TUS
// specifications to support resumable file upload and extends the handler to
// support fetching media from remote url.
type Handler struct {
	*tusd.UnroutedHandler
	// config   *Launcher
	queries *database.Queries
	// composer *tusd.StoreComposer
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

func (c *Launcher) HTTPAddress(address string) *Launcher {
	c.httpAddress = address
	return c
}

func (c *Launcher) DB(db database.DBTX) *Launcher {
	c.db = db
	return c
}

func (l *Launcher) Build() (chi.Router, error) {
	// tokenAuth := jwtauth.New("ES256", nil, l.publicKey)
	validator, err := keybox.NewValidator(l.publicKey)
	if err != nil {
		return nil, err
	}

	l.logger.Info("building upload handler")
	readyCtx, readyCancel := context.WithCancel(context.Background())
	handler := &Handler{
		logger:  l.logger,
		queries: database.New(l.db),
		// jwtAuth:  tokenAuth,
		tokenValidator: validator,
	}
	l.readyCancel = readyCancel

	composer := tusd.NewStoreComposer()
	composer.UseLocker(l.fileLocker)
	l.s3Store.UseIn(composer)

	tusConfig := tusd.Config{
		StoreComposer: composer,
		BasePath:      l.prefix,
		// PreUploadCreateCallback: handler.preCreateHook,
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
	// router.Use(middleware.Logger)
	router.Use(httpLogger.Middleware)
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
		// r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(handler.authByToken)
		r.Use(handler.Middleware)
		// r.MethodFunc("OPTIONS", "/", handler.OptionsFile)
		r.Post("/", handler.PostFile)
		r.Head("/*", handler.HeadFile)
		r.Patch("/*", handler.PatchFile)
		r.Delete("/*", handler.DelFile)
		// Notify?
	})

	httpServer := &http.Server{
		Addr: l.httpAddress,
		// Handler: handlers.LoggingHandler(os.Stdout, router),
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
// func (h *Handler) preCreateHook(hook tusd.HookEvent) error {
// 	token, err := jwtauth.VerifyRequest(
// 		h.jwtAuth, &http.Request{Header: hook.HTTPRequest.Header}, jwtauth.TokenFromHeader)
// 	if err != nil {
// 		return fmt.Errorf("cannot authenticate user: %w", err)
// 	}
// 	if token.Subject() == "" {
// 		return fmt.Errorf("missing user id in token")
// 	}
// 	// if _, err := h.queries.CreateUpload(context.Background(), database.CreateUploadParams{
// 	// 	UserID: uid,
// 	// 	ID:     hook.Upload.ID,
// 	// 	Size:   hook.Upload.Size,
// 	// }); err != nil {
// 	// 	return fmt.Errorf("upload record creation failed: %w", err)
// 	// }
// 	return nil
// }

func (h *Handler) listenToHooks() {
	h.logger.Info("listening to upload signals")

	listen := func(ch chan tusd.HookEvent, eventHandler func(string, tusd.HookEvent)) {
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

	go listen(h.CreatedUploads, func(uid string, event tusd.HookEvent) {
		p := database.CreateUploadParams{
			UserID: uid,
			ID:     event.Upload.ID,
			Size:   event.Upload.Size,
		}
		_, err := h.queries.CreateUpload(context.Background(), p)
		if err != nil {
			h.logger.Warn("creating upload failed", "user_id", uid, "upload_id", p.ID, "err", err)
		}
		h.logger.Info("upload created", "user_id", uid, "upload_id", p.ID)
	})

	go listen(h.UploadProgress, func(uid string, event tusd.HookEvent) {
		p := database.RecordUploadProgressParams{
			UserID:   uid,
			ID:       event.Upload.ID,
			Received: event.Upload.Offset,
		}
		err := h.queries.RecordUploadProgress(context.Background(), p)
		if err != nil {
			h.logger.Warn("recording upload progress failed", "user_id", p.UserID, "upload_id", p.ID, "err", err)
		}
		h.logger.Debug("upload progress", "user_id", uid, "upload_id", p.ID, "size", event.Upload.Size, "received", p.Received)
	})

	go listen(h.TerminatedUploads, func(uid string, event tusd.HookEvent) {
		p := database.MarkUploadTerminatedParams{
			UserID: uid,
			ID:     event.Upload.ID,
		}
		err := h.queries.MarkUploadTerminated(context.Background(), p)
		if err != nil {
			h.logger.Warn("recording upload termination failed", "user_id", uid, "upload_id", p.ID, "err", err)
		}
		h.logger.Info("upload terminated", "user_id", uid, "upload_id", p.ID)
	})

	go listen(h.CompleteUploads, func(uid string, event tusd.HookEvent) {
		p := database.MarkUploadCompletedParams{
			UserID:   uid,
			ID:       event.Upload.ID,
			Filename: event.Upload.MetaData["filename"],
			Key:      event.Upload.Storage["Key"],
		}
		err := h.queries.MarkUploadCompleted(context.Background(), p)
		if err != nil {
			h.logger.Warn("recording upload completion failed", "user_id", uid, "upload_id", p.ID, "err", err)
		}
		h.logger.Info(
			"upload completed",
			"user_id", uid, "upload_id", p.ID, "filename", p.Filename, "size", event.Upload.Size)
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

func (h *Handler) extractUserIDFromRequest(r *http.Request) (string, error) {
	rt := jwtauth.TokenFromHeader(r)
	token, err := h.tokenValidator.ParseToken(rt)
	if err != nil {
		return "", err
	}

	if err := jwt.Validate(token); err != nil {
		return "", fmt.Errorf("cannot validate token: %w", err)
	}
	if token.Subject() == "" {
		return "", errors.New("missing user id in token")
	}
	return token.Subject(), nil
}

func extractUserIDFromPath(url string) string {
	result := reExtractUserID.FindStringSubmatch(url)
	if len(result) != 2 {
		return ""
	}
	return result[1]
}

// extractUploadIDFromPath pulls the last segment from the url provided
func extractUploadIDFromPath(url string) string {
	result := reExtractFileID.FindStringSubmatch(url)
	if len(result) != 2 {
		return ""
	}
	return result[1]
}
