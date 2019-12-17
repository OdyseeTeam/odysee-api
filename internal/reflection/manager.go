package reflection

import (
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/lbryio/lbry.go/v2/extras/errors"
	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/lbryio/reflector.go/reflector"
)

var logger = monitor.NewModuleLogger("reflection")

// Manager represents an object for managing and scheduling published data upload to reflectors.
type Manager struct {
	blobsPath       string
	reflector       string
	uploader        *reflector.Client
	stopChan        chan bool
	abortUploadChan chan bool
	isInitialized   bool
}

// ReflError contains a blob file name and an error
type ReflError struct {
	FilePath string
	Error    error
}

// RunStats contains stats of blob reflection run, typically a result of ReflectAll call
type RunStats struct {
	TotalBlobs     int
	ReflectedBlobs int
	Errors         []ReflError
}

// NewManager returns a Manager instance.
// To initialize a returned instance (connect to the reflector DB), call Initialize() on it.
func NewManager(blobsPath string, reflector string) *Manager {
	return &Manager{
		blobsPath: blobsPath,
		reflector: reflector,
		stopChan:  make(chan bool),
	}
}

// Initialize connects to the reflector database
func (r *Manager) Initialize() {
	c := reflector.Client{}
	r.uploader = &c

	err := r.uploader.Connect(r.reflector)
	if err != nil {
		logger.Log().Errorf("reflection was NOT initialized: %v", err)
		return
	}
	defer r.uploader.Close()

	f, err := os.Open(r.blobsPath)
	if err != nil {
		logger.Log().Errorf("reflection was NOT initialized: %v", err)
		return
	}
	defer f.Close()

	r.isInitialized = true
	logger.Log().Infof("manager initialized")
}

// IsInitialized returns true whenever Manager object is ready to use.
func (r *Manager) IsInitialized() bool {
	return r.isInitialized
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
			case <-r.stopChan:
				logger.Log().Info("stopping reflection...")
				ticker.Stop()
				logger.Log().Info("stopped")
				return
			case <-ticker.C:
				stats, err := r.ReflectAll()
				if err != nil {
					logger.Log().Errorf("failed to reflect blobs: %v", err)
				} else {
					logger.Log().Infof(
						"total blob: %v, reflected/removed: %v, errors encountered: %v",
						stats.TotalBlobs, stats.ReflectedBlobs, len(stats.Errors),
					)
					for _, e := range stats.Errors {
						logger.Log().Errorf("blob %v: %v", e.FilePath, e.Error)
					}
				}
			}
		}
	}()
}

// Abort resets the upload schedule and cancels blob upload
// after the currently uploading blob is finished.
func (r *Manager) Abort() {
	r.stopChan <- true
	r.abortUploadChan <- true
}

// ReflectAll uploads and then deletes all blobs in the blob directory.
func (r *Manager) ReflectAll() (*RunStats, error) {
	pendingFilenames := []string{}
	stats := &RunStats{}
	log := logger.Log()

	log.Debugf("checking %v for blobs...", r.blobsPath)
	f, err := os.Open(r.blobsPath)
	if err != nil {
		return nil, err
	}

	entries, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}

	for _, file := range entries {
		if !file.IsDir() {
			pendingFilenames = append(pendingFilenames, path.Join(r.blobsPath, file.Name()))
		}
	}
	stats.TotalBlobs = len(pendingFilenames)
	log.Debugf("%v blobs found", stats.TotalBlobs)

	err = r.uploader.Connect(r.reflector)
	if err != nil {
		return nil, err
	}

	for _, f := range pendingFilenames {
		select {
		case <-r.abortUploadChan:
			break
		default:
		}

		b, err := ioutil.ReadFile(f)
		if err != nil {
			stats.Errors = append(stats.Errors, ReflError{f, err})
			continue
		}

		err = r.uploader.SendBlob(stream.Blob(b))
		if errors.Is(err, reflector.ErrBlobExists) || err == nil {
			stats.ReflectedBlobs++
			if err := os.Remove(f); err != nil {
				stats.Errors = append(stats.Errors, ReflError{f, err})
			}
		} else {
			stats.Errors = append(stats.Errors, ReflError{f, err})
		}
	}
	log.Debug("reflection run complete")

	return stats, nil
}
