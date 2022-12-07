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

	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/pkg/blobs"
	"github.com/OdyseeTeam/odysee-api/pkg/fileanalyzer"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/hibiken/asynq"
	"github.com/ybbus/jsonrpc"
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

type UploadHandler struct {
	blobsDstPath string
	analyzer     *fileanalyzer.Analyzer
	store        *blobs.Store
	results      chan<- UploadResult
	logger       logging.KVLogger
}

func NewUploadHandler(blobsDstPath string, results chan<- UploadResult, reflectorCfg map[string]string, logger logging.KVLogger) (*UploadHandler, error) {
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

	c := &UploadHandler{
		store:        s,
		analyzer:     analyzer,
		blobsDstPath: blobsDstPath,
		results:      results,
		logger:       logger,
	}
	return c, nil
}

func (c *UploadHandler) HandleTask(ctx context.Context, task *asynq.Task) error {
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
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingAnalyze).Observe(float64(time.Since(t)))
	if ad == nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingAnalyze).Inc()
		log.Warn("file analysis failed", "err", err, "file_path", p.Path)
		return fmt.Errorf("file analysis failed: %w", err)
	}
	log.Debug("stream analyzed", "result", ad, "err", err)

	src := blobs.NewSource(p.Path, c.blobsDstPath)
	t = time.Now()
	stream, err := src.Split()
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingBlobSplit).Observe(float64(time.Since(t)))
	if err != nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingBlobSplit).Inc()
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
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingReflection).Observe(float64(time.Since(t)))
	if err != nil {
		// The errors current uploader returns us ually do not make sense to retry.
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingReflection).Inc()
		log.Warn("blobs upload failed, not retrying", "err", err, "blobs_path", c.blobsDstPath)
		return asynq.SkipRetry
	} else if summary.Err > 0 {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingReflection).Inc()
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

func (c *UploadHandler) RetryDelay(n int, err error, t *asynq.Task) time.Duration {
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
