package reflection

import (
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/lbryio/lbry.go/v2/extras/errors"
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
	sync.RWMutex
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
	start := time.Now()

	logger.Log().Infof("starting reflection")

	pendingFilenames := []string{}
	stats := &RunStats{}

	logger.Log().Debugf("checking %v for blobs...", r.blobsPath)
	f, err := os.Open(r.blobsPath)
	if err != nil {
		return nil, errors.Err(err)
	}

	entries, err := f.Readdir(-1)
	if err != nil {
		return nil, errors.Err(err)
	}
	err = f.Close()
	if err != nil {
		return nil, errors.Err(err)
	}

	for _, file := range entries {
		if !file.IsDir() {
			pendingFilenames = append(pendingFilenames, path.Join(r.blobsPath, file.Name()))
		}
	}
	stats.TotalBlobs = len(pendingFilenames)
	logger.Log().Debugf("%v blobs found", stats.TotalBlobs)

	var wg sync.WaitGroup
	wgSize := 10

	for i, f := range pendingFilenames {
		select {
		case <-r.abortUploadChan:
			break
		default:
		}

		wg.Add(1)
		go func(wf string) {
			rc := reflector.Client{}
			err := rc.Connect(r.reflector)
			if err != nil {
				return
			}
			b, err := ioutil.ReadFile(wf)
			if err != nil {
				stats.Lock()
				stats.Errors = append(stats.Errors, ReflError{wf, err})
				stats.Unlock()
			}

			err = rc.SendBlob(b)
			if errors.Is(err, reflector.ErrBlobExists) || err == nil {
				stats.Lock()
				stats.ReflectedBlobs++
				stats.Unlock()
				if err := os.Remove(wf); err != nil {
					stats.Lock()
					stats.Errors = append(stats.Errors, ReflError{wf, err})
					stats.Unlock()
				}
			} else {
				stats.Lock()
				stats.Errors = append(stats.Errors, ReflError{wf, err})
				stats.Unlock()
			}
			wg.Done()
		}(f)
		if (i%wgSize) == 0 || len(pendingFilenames)-1-i == 0 {
			wg.Wait()
		}
	}

	logger.Log().Infof("reflection run complete in %.2f minutes", time.Since(start).Minutes())

	return stats, nil
}
