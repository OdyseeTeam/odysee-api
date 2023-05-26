package asynquery

import (
	"context"
	"crypto"

	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
	"github.com/volatiletech/sqlboiler/boil"
)

type Launcher struct {
	busRedisOpts asynq.RedisConnOpt
	db           boil.Executor
	logger       logging.KVLogger
	manager      *CallManager
	privateKey   crypto.PrivateKey
	readyCancel  context.CancelFunc
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
func WithBusRedisOpts(redisOpts asynq.RedisConnOpt) LauncherOption {
	return func(l *Launcher) {
		l.busRedisOpts = redisOpts
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
	l.logger.Info("installing routes")
	keyfob, err := keybox.NewKeyfob(l.privateKey)
	if err != nil {
		return err
	}
	manager, err := NewCallManager(l.busRedisOpts, l.db, l.logger)
	if err != nil {
		return err
	}
	l.manager = manager
	handler := NewHandler(manager, l.logger, keyfob)
	r.HandleFunc("/api/v1/asynqueries/auth/pubkey", keybox.PublicKeyHandler(keyfob)).Methods("GET")
	r.HandleFunc("/api/v1/asynqueries/auth/upload-token", handler.RetrieveUploadToken).Methods("POST")
	r.HandleFunc("/api/v1/asynqueries/{id}", handler.Get).Methods("GET")
	r.HandleFunc("/api/v1/asynqueries/", handler.Create).Methods("POST")
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
