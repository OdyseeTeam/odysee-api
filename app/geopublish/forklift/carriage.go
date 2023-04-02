package forklift

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/blobs"
	"github.com/OdyseeTeam/odysee-api/pkg/fileanalyzer"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/hibiken/asynq"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"
)

// A list of task types.
const (
	TypeUploadProcess = "upload:process"
)

var ErrUpload = errors.New("several errors detected uploading blobs")

type UploadProcessPayload struct {
	UploadID string
	Path     string
	UserID   int
	Request  *jsonrpc.RPCRequest
}

type UploadProcessResult struct {
	UploadID string
	UserID   int
	SDHash   string
	Response *jsonrpc.RPCResponse
	Error    string
	Retry    bool
}

type Carriage struct {
	blobsPath    string
	analyzer     *fileanalyzer.Analyzer
	store        *blobs.Store
	resultWriter io.Writer
	logger       logging.KVLogger
}

func NewCarriage(blobsPath string, resultWriter io.Writer, reflectorCfg map[string]string, logger logging.KVLogger) (*Carriage, error) {
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

	c := &Carriage{
		store:        s,
		analyzer:     analyzer,
		blobsPath:    blobsPath,
		resultWriter: resultWriter,
		logger:       logger,
	}
	return c, nil
}

func (c *Carriage) ProcessTask(ctx context.Context, t *asynq.Task) error {
	if t.Type() != TypeUploadProcess {
		c.logger.Warn("cannot handle task", "type", t.Type())
		return fmt.Errorf("cannot handle %s", t.Type())
	}
	var p UploadProcessPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	r, err := c.Process(p)
	if err != nil {
		r.Error = err.Error()
	}
	br, err := json.Marshal(r)
	if err != nil {
		c.logger.Error("error serializing processing result", "err", err, "result", r)
		return fmt.Errorf("error serializing processing result: %s (%w)", err, asynq.SkipRetry)
	}

	if r.Error != "" {
		perr := fmt.Errorf("upload processing failed: %s", r.Error)
		if r.Retry {
			c.logger.Warn("upload processing failed", "result", r, "payload", p)
			return perr
		}
		c.logger.Error("upload processing failed fatally", "result", r, "payload", p)
		_, err = c.resultWriter.Write(br)
		if err != nil {
			perr = fmt.Errorf("%s (also error writing result: %w)", perr, err)
		}
		return fmt.Errorf("%s (%w)", perr, asynq.SkipRetry)
	}

	_, err = c.resultWriter.Write(br)
	if err != nil {
		c.logger.Error("writing result failed", "err", err)
		return fmt.Errorf("error writing result: %w", err)
	}
	return nil
}

func (c *Carriage) Process(p UploadProcessPayload) (*UploadProcessResult, error) {
	var t time.Time
	r := &UploadProcessResult{UploadID: p.UploadID, UserID: p.UserID}
	log := c.logger.With("upload_id", p.UploadID, "user_id", p.UserID)

	uploader := c.store.Uploader()

	t = time.Now()
	info, err := c.analyzer.Analyze(context.Background(), p.Path)
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingAnalyze).Observe(float64(time.Since(t)))
	if info == nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingAnalyze).Inc()
		return r, err
	}
	log.Debug("stream analyzed", "info", info, "err", err)

	src, err := blobs.NewSource(p.Path, c.blobsPath)
	if err != nil {
		return r, err
	}

	t = time.Now()
	stream, err := src.Split()
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingBlobSplit).Observe(float64(time.Since(t)))
	if err != nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingBlobSplit).Inc()
		return r, err
	}
	streamSource := stream.GetSource()
	r.SDHash = hex.EncodeToString(streamSource.GetSdHash())
	defer os.RemoveAll(path.Join(c.blobsPath, r.SDHash))

	t = time.Now()
	summary, err := uploader.Upload(src)
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingReflection).Observe(float64(time.Since(t)))
	if err != nil {
		// The errors current uploader returns usually do not make sense to retry.
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingReflection).Inc()
		return r, err
	} else if summary.Err > 0 {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingReflection).Inc()
		r.Retry = true
		return r, fmt.Errorf("%w (%v)", ErrUpload, summary.Err)
	}

	u, err := models.Users(
		models.UserWhere.ID.EQ(p.UserID),
		qm.Load(models.UserRels.LbrynetServer),
	).OneG()
	if err != nil {
		return r, fmt.Errorf("error getting sdk address for user %v: %w", p.UserID, err)
	}

	caller := query.NewCaller(u.R.LbrynetServer.Address, p.UserID)

	fileName := streamSource.Name
	if path.Ext(streamSource.Name) == "" {
		fileName += info.MediaType.Extension
	}

	patch := map[string]interface{}{
		"file_size": streamSource.Size,
		"file_name": fileName,
		"file_hash": hex.EncodeToString(streamSource.GetHash()),
		"sd_hash":   r.SDHash,
	}

	m := info.MediaInfo
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

	log.Debug("sending request", "method", p.Request.Method, "params", p.Request)
	t = time.Now()
	res, err := caller.Call(context.Background(), p.Request)
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingQuery).Observe(float64(time.Since(t)))

	metrics.QueriesSent.Inc()
	if err != nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingQuery).Inc()
		metrics.QueriesFailed.Inc()
		r.Retry = true
		return r, fmt.Errorf("error calling sdk: %w", err)
	}

	r.Response = res
	if res.Error != nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingQuery).Inc()
		metrics.QueriesErrored.Inc()
		return r, fmt.Errorf("sdk returned an error: %s", res.Error.Message)
	}
	metrics.QueriesCompleted.Inc()
	log.Info("stream processed", "method", p.Request.Method, "params", p.Request)
	return r, nil
}

func (c *Carriage) RetryDelay(n int, err error, t *asynq.Task) time.Duration {
	if errors.Is(err, ErrUpload) {
		return time.Duration(n) * time.Minute
	}
	return 10 * time.Second
}
