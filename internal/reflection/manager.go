package reflection

import (
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/lbryio/reflector.go/db"
	"github.com/lbryio/reflector.go/reflector"
	"github.com/lbryio/reflector.go/store"
)

var logger = monitor.NewModuleLogger("reflection")

type Manager struct {
	uploader *reflector.Uploader
	client   *ljsonrpc.Client
	dbHandle *db.SQL
	config   config.ReflectorConfig
	abort    chan bool
}

func NewManager(rCfg config.ReflectorConfig) *Manager {
	return &Manager{
		abort:  make(chan bool),
		config: rCfg,
	}
}

func (r *Manager) Initialize() {
	db := new(db.SQL)
	err := db.Connect(r.config.DBConn)
	if err != nil {
		logger.Log().Errorf("reflection was NOT initialized, cannot connect to reflector database: %v", err)
		return
	}
	r.dbHandle = db
	logger.Log().Infof("manager initialized")
}

func (r *Manager) IsInitialized() bool {
	return r.dbHandle != nil
}

func (r *Manager) Start(interval time.Duration) {
	if !r.IsInitialized() {
		return
	}
	logger.Log().Infof("starting reflection schedule (every %v minutes)", interval.Minutes())
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-r.abort:
				logger.Log().Info("stopping reflection...")
				ticker.Stop()
				r.uploader.Stop()
				logger.Log().Info("stopped")
				return
			case <-ticker.C:
				r.ReflectAll()
			}
		}
	}()
}

func (r *Manager) Abort() {
	r.abort <- true
}

func (r *Manager) ReflectAll() {
	if !r.IsInitialized() {
		return
	}

	var err error

	st := store.NewDBBackedS3Store(
		store.NewS3BlobStore(r.config.AWSID, r.config.AWSSecret, r.config.Region, r.config.Bucket),
		r.dbHandle)

	uploadWorkers := 10
	uploader := reflector.NewUploader(r.dbHandle, st, uploadWorkers, false)

	err = uploader.Upload(config.GetBlobFilesDir())
	if err != nil {
		logger.Log().Error(err)
		monitor.CaptureException(err)
	}

	summary := uploader.GetSummary()
	if summary.Err > 0 {
		logger.Log().Errorf("some blobs were not uploaded: %v (total: %v)", summary.Err, summary.Total)
		monitor.CaptureException(fmt.Errorf("some blobs were not uploaded: %v (total: %v)", summary.Err, summary.Total))
	} else {
		logger.Log().Infof("uploaded %v blobs", summary.Blob)
	}
}

func (r *Manager) cleanupBlobs() {

}
