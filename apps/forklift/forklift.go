package forklift

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/asynquery"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/pkg/belt"
	"github.com/OdyseeTeam/odysee-api/pkg/blobs"
	"github.com/OdyseeTeam/odysee-api/pkg/fileanalyzer"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/hibiken/asynq"
	"github.com/ybbus/jsonrpc"
)

// A list of task types.
const (
	TaskUpload       = "forklift:upload"
	TaskUploadResult = "forklift:result"
)

var ErrReflector = errors.New("errors found uploading blobs to reflector")

type UploadPayload struct {
	UploadID string
	QueryID  int
	Path     string
	UserID   int
	Request  *jsonrpc.RPCRequest
}

type UploadResult struct {
	UploadID string
	QueryID  int
	UserID   int
	Error    string
	Request  *jsonrpc.RPCRequest
}

type Forklift struct {
	blobsDstPath string
	analyzer     *fileanalyzer.Analyzer
	store        *blobs.Store
	results      chan<- UploadResult
	logger       logging.KVLogger
}

func Start(blobsDstPath string, logger logging.KVLogger) (*belt.Belt, error) {
	if logger == nil {
		logger = zapadapter.NewKV(nil)
	}

	ro, err := config.GetAsynqRedisOpts()
	if err != nil {
		logger.Fatal("unable to get redis config", "err", err)
	}

	m, err := asynquery.NewCallManager(ro, zapadapter.NewKV(nil))
	if err != nil {
		logger.Fatal("unable to start asynquery manager", "err", err)
	}

	results := make(chan UploadResult)
	f, err := NewForklift(blobsDstPath, config.GetReflectorUpstream(), results, logger)
	if err != nil {
		logger.Fatal("unable to start forklift", "err", err)
	}
	b, err := belt.New(ro, belt.WithLogger(logger), belt.WithConcurrency(config.GetGeoPublishConcurrency()))
	if err != nil {
		logger.Fatal("unable to start task belt", "err", err)
	}
	b.AddHandler(TaskUpload, f.HandleTask)

	go func() {
		for r := range results {
			if r.Error != "" {
				b.Put(TaskUploadResult, r, 10)
				continue
			}

			m.Add(r.UserID, r.Request)
		}
	}()

	return b, nil
}

func NewForklift(blobsDstPath string, reflectorCfg map[string]string, results chan<- UploadResult, logger logging.KVLogger) (*Forklift, error) {
	analyzer, err := fileanalyzer.NewAnalyzer()
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = zapadapter.NewKV(nil)
	}

	s, err := blobs.NewStore(reflectorCfg)
	if err != nil {
		return nil, err
	}

	c := &Forklift{
		store:        s,
		analyzer:     analyzer,
		blobsDstPath: blobsDstPath,
		results:      results,
		logger:       logger,
	}
	return c, nil
}

func (c *Forklift) HandleTask(ctx context.Context, task *asynq.Task) error {
	if task.Type() != TaskUpload {
		c.logger.Warn("cannot handle task", "type", task.Type())
		return asynq.SkipRetry
	}
	var p UploadPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		c.logger.Warn("message unmarshal failed", "err", err)
		return asynq.SkipRetry
	}

	log := logging.TracedLogger(c.logger, p)
	var t time.Time

	result := UploadResult{UploadID: p.UploadID, UserID: p.UserID}

	uploader := c.store.Uploader()

	t = time.Now()
	ad, err := c.analyzer.Analyze(context.Background(), p.Path)
	ProcessingTime.WithLabelValues(LabelProcessingAnalyze).Observe(float64(time.Since(t)))
	if ad == nil {
		ProcessingErrors.WithLabelValues(LabelProcessingAnalyze).Inc()
		log.Warn("file analysis failed", "err", err, "file_path", p.Path)
		return fmt.Errorf("file analysis failed: %w", err)
	}
	log.Debug("stream analyzed", "result", ad, "err", err)

	src := blobs.NewSource(p.Path, c.blobsDstPath)
	t = time.Now()
	stream, err := src.Split()
	ProcessingTime.WithLabelValues(LabelProcessingBlobSplit).Observe(float64(time.Since(t)))
	if err != nil {
		ProcessingErrors.WithLabelValues(LabelProcessingBlobSplit).Inc()
		log.Warn("failed to split stream", "err", err, "file_path", p.Path, "blobs_path", c.blobsDstPath)
		return fmt.Errorf("failed to split stream: %w", err)
	}
	log.Info("blobs generated", "duration", time.Since(t).Seconds())

	streamSource := stream.GetSource()
	sdHash := hex.EncodeToString(streamSource.GetSdHash())
	defer func() {
		err := os.RemoveAll(path.Join(c.blobsDstPath, sdHash))
		if err != nil {
			log.Warn("failed to remove blobs", "err", err)
		}
	}()

	t = time.Now()
	summary, err := uploader.Upload(src)
	ProcessingTime.WithLabelValues(LabelProcessingReflection).Observe(float64(time.Since(t)))
	if err != nil {
		// The errors current uploader returns us ually do not make sense to retry.
		ProcessingErrors.WithLabelValues(LabelProcessingReflection).Inc()
		log.Warn("blobs upload failed, not retrying", "err", err, "blobs_path", c.blobsDstPath)
		return asynq.SkipRetry
	} else if summary.Err > 0 {
		ProcessingErrors.WithLabelValues(LabelProcessingReflection).Inc()
		log.Warn(ErrReflector.Error(), "err_count", summary.Err, "blobs_path", c.blobsDstPath)
		result.Error = fmt.Sprintf("%d errors encountered", summary.Err)
		c.results <- result
		return ErrReflector
	}

	fileName := streamSource.Name
	if path.Ext(streamSource.Name) == "" {
		fileName += ad.MediaType.Extension
	}

	// Patch the original SDK request
	patch := map[string]interface{}{
		"file_size": streamSource.Size,
		"file_name": fileName,
		"file_hash": hex.EncodeToString(streamSource.GetHash()),
		"sd_hash":   sdHash,
	}

	m := ad.MediaInfo
	if m != nil {
		patch["width"] = m.Width
		patch["height"] = m.Height
		patch["duration"] = m.Duration
	}
	pp := p.Request.Params.(map[string]interface{})
	for k, v := range patch {
		pp[k] = v
	}
	delete(pp, "file_path")

	c.results <- result

	return nil
}

func (c *Forklift) RetryDelay(n int, err error, t *asynq.Task) time.Duration {
	if errors.Is(err, ErrReflector) {
		return time.Duration(n) * time.Minute
	}
	return 10 * time.Second
}

func (p UploadPayload) GetTraceData() map[string]string {
	return map[string]string{
		"user_id":   strconv.Itoa(p.UserID),
		"upload_id": p.UploadID,
	}
}

func (p UploadResult) GetTraceData() map[string]string {
	return map[string]string{
		"user_id":   strconv.Itoa(p.UserID),
		"upload_id": p.UploadID,
	}
}
