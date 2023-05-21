package asynquery

import (
	"context"
	"crypto"

	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
)

type Launcher struct {
	privateKey  crypto.PrivateKey
	busRedisURL string
	logger      logging.KVLogger
	readyCancel context.CancelFunc
}

type LauncherOption func(*Launcher)

func WithPrivateKey(privateKey crypto.PrivateKey) LauncherOption {
	return func(l *Launcher) {
		l.privateKey = privateKey
	}
}

func WithBusRedisURL(busRedisURL string) LauncherOption {
	return func(l *Launcher) {
		l.busRedisURL = busRedisURL
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

func NewLauncher(options ...LauncherOption) *Launcher {
	launcher := &Launcher{
		logger: logging.NoopKVLogger{},
	}
	for _, option := range options {
		option(launcher)
	}
	return launcher
}

func (l *Launcher) InstallRoutes(r *mux.Router) error {
	keyfob, err := keybox.NewKeyfob(l.privateKey)
	if err != nil {
		return err
	}
	redisOpts, err := asynq.ParseRedisURI(l.busRedisURL)
	if err != nil {
		return err
	}
	cm, err := NewCallManager(redisOpts, l.logger)
	if err != nil {
		return err
	}
	handler := NewHandler(cm, l.logger, keyfob)
	r.HandleFunc("/api/v1/asynqueries/auth/pubkey", keybox.PublicKeyHandler(keyfob)).Methods("GET")
	r.HandleFunc("/api/v1/asynqueries/auth/upload-token", handler.RetrieveUploadToken).Methods("POST")
	r.HandleFunc("/api/v1/asynqueries", handler.Create).Methods("POST")
	r.HandleFunc("/api/v1/asynqueries/{id}", handler.Get).Methods("GET")
	return nil
}
