package geopublish

import (
	"fmt"
	"net/http"
	"path"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/forklift"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/pkg/redislocker"

	"github.com/gorilla/mux"
	"github.com/tus/tusd/pkg/filestore"
	tushandler "github.com/tus/tusd/pkg/handler"
)

func InstallRoutes(router *mux.Router, userGetter UserGetter, uploadPath, urlPrefix string) (*Handler, error) {
	redisOpts, err := config.GetRedisOpts()
	if err != nil {
		return nil, fmt.Errorf("cannot get redis config: %w", err)
	}
	asynqRedisOpts, err := config.GetAsynqRedisOpts()
	if err != nil {
		return nil, fmt.Errorf("cannot get redis config: %w", err)
	}

	composer := tushandler.NewStoreComposer()
	store := filestore.New(uploadPath)
	store.UseIn(composer)

	fl, err := forklift.NewForklift(
		path.Join(uploadPath, "blobs"),
		config.GetReflectorUpstream(),
		asynqRedisOpts,
		forklift.WithConcurrency(config.GetGeoPublishConcurrency()),
		// forklift.WithLogger(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize forklift: %w", err)
	}
	err = fl.Start()
	if err != nil {
		return nil, fmt.Errorf("cannot start forklift: %w", err)
	}

	locker, err := redislocker.New(redisOpts)
	if err != nil {
		return nil, fmt.Errorf("cannot start redislocker: %w", err)
	}
	locker.UseIn(composer)

	tusCfg := tushandler.Config{
		BasePath:      urlPrefix,
		StoreComposer: composer,
	}

	tusHandler, err := NewHandler(
		WithUserGetter(userGetter),
		WithTusConfig(tusCfg),
		WithUploadPath(uploadPath),
		WithDB(storage.DB),
		WithRedisOpts(asynqRedisOpts),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize tus handler: %w", err)
	}

	r := router
	r.Use(tusHandler.Middleware)
	r.HandleFunc("/", tusHandler.PostFile).Methods(http.MethodPost).Name("geopublish")
	r.HandleFunc("/{id}", tusHandler.HeadFile).Methods(http.MethodHead)
	r.HandleFunc("/{id}", tusHandler.PatchFile).Methods(http.MethodPatch)
	r.HandleFunc("/{id}", tusHandler.DelFile).Methods(http.MethodDelete)
	r.HandleFunc("/{id}/notify", tusHandler.Notify).Methods(http.MethodPost)
	r.HandleFunc("/{id}/status", tusHandler.Status).Methods(http.MethodGet)
	r.PathPrefix("/").HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}).Methods(http.MethodOptions)

	return tusHandler, nil
}
