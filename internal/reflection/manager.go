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

// Manager represents an object for managing and scheduling published data upload to reflectors.
type Manager struct {
	uploader *reflector.Uploader
	client   *ljsonrpc.Client
	dbHandle *db.SQL
	config   config.ReflectorConfig
	abort    chan bool
}

// NewManager returns a Manager instance.
// To initialize a returned instance (connect to the reflector DB), call Initialize() on it.
func NewManager(rCfg config.ReflectorConfig) *Manager {
	return &Manager{
		abort:  make(chan bool),
		config: rCfg,
	}
}

// Initialize connects to the reflector database
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

// IsInitialized returns true whenever Manager object is ready to use.
func (r *Manager) IsInitialized() bool {
	return r.dbHandle != nil
}

// Start launches blob upload procedure at specified intervals.
// If upload duration at the end exceeds specified interval, it will just start the upload again after it's done.
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

// Abort resets the upload schedule and cancels current upload.
func (r *Manager) Abort() {
	r.abort <- true
}

// ReflectAll starts an upload process for all blobs in the specified directory.
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

// CleanupBlobs checks which of the blobs are present in the reflector database already and removes them
// both from local SDK instance and from the filesystem.
func (r *Manager) CleanupBlobs() {

}
