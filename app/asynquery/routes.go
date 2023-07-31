package asynquery

import (
	"context"
	"crypto"
	"errors"
	"net/http"

	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
	"github.com/volatiletech/sqlboiler/boil"
)

type Launcher struct {
	requestsConnOpts asynq.RedisConnOpt
	db               boil.Executor
	logger           logging.KVLogger
	manager          *CallManager
	privateKey       crypto.PrivateKey
	readyCancel      context.CancelFunc
	uploadServiceURL string
}

type LauncherOption func(*Launcher)

func WithPrivateKey(privateKey crypto.PrivateKey) LauncherOption {
	return func(l *Launcher) {
		l.privateKey = privateKey
	}
}

func WithDB(db boil.Executor) LauncherOption {
	return func(l *Launcher) {
		l.db = db
	}
}
func WithRequestsConnOpts(redisOpts asynq.RedisConnOpt) LauncherOption {
	return func(l *Launcher) {
		l.requestsConnOpts = redisOpts
	}
}

func WithLogger(logger logging.KVLogger) LauncherOption {
	return func(l *Launcher) {
		l.logger = logger
	}
}

func WithReadyCancel(readyCancel context.CancelFunc) LauncherOption {
	return func(l *Launcher) {
		l.readyCancel = readyCancel
	}
}

func WithUploadServiceURL(url string) LauncherOption {
	return func(l *Launcher) {
		l.uploadServiceURL = url
	}
}

func NewLauncher(options ...LauncherOption) *Launcher {
	launcher := &Launcher{
		logger:           logging.NoopKVLogger{},
		uploadServiceURL: "https://uploads-v4.na-backend.odysee.com/v1/",
	}
	for _, option := range options {
		option(launcher)
	}
	return launcher
}

func (l *Launcher) InstallRoutes(r *mux.Router) error {
	l.logger.Info("installing routes")
	if l.requestsConnOpts == nil {
		return errors.New("missing redis requests connection options")
	}
	keyfob, err := keybox.NewKeyfob(l.privateKey)
	if err != nil {
		return err
	}
	manager, err := NewCallManager(l.requestsConnOpts, l.db, l.logger)
	if err != nil {
		return err
	}
	l.manager = manager
	handler := NewHandler(manager, l.logger, keyfob, l.uploadServiceURL)
	r = r.PathPrefix("/asynqueries").Subrouter()
	r.PathPrefix("/").HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}).Methods(http.MethodOptions)
	r.HandleFunc("/auth/pubkey", keyfob.PublicKeyHandler).Methods("GET")
	r.HandleFunc("/{type:(?:urls)|(?:uploads)}/", handler.CreateUpload).Methods("POST")
	r.HandleFunc("/{id}", handler.Get).Methods("GET")
	r.HandleFunc("/", handler.CreateQuery).Methods("POST")
	l.logger.Info("routes installed")
	return nil
}

func (l *Launcher) Start() error {
	err := l.manager.Start()
	if err != nil {
		l.logger.Error("failed to start asynquery manager", "err", err)
		return err
	}
	return nil
}

func (l *Launcher) Shutdown() {
	l.manager.Shutdown()
}
