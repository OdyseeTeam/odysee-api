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
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc/v2"
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

func NewCarriage(blobsPath string, resultWriter io.Writer, reflectorConfig *viper.Viper, logger logging.KVLogger) (*Carriage, error) {
	analyzer, err := fileanalyzer.NewAnalyzer()
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = zapadapter.NewKV(nil)
	}

	destinations, err := blobs.CreateStoresFromConfig(reflectorConfig, "destinations")
	if err != nil {
		return nil, fmt.Errorf("cannot initialize reflector destination stores: %w", err)
	}
	store, err := blobs.NewStore(reflectorConfig.GetString("databasedsn"), destinations)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize reflector store: %w", err)
	}

	c := &Carriage{
		store:        store,
		analyzer:     analyzer,
		blobsPath:    blobsPath,
		resultWriter: resultWriter,
		logger:       logger,
	}
	return c, nil
}

func (c *Carriage) ProcessTask(ctx context.Context, t *asynq.Task) error {
	if t.Type() != TypeUploadProcess {
		c.logger.Error("cannot handle task", "type", t.Type())
		return fmt.Errorf("cannot handle %s", t.Type())
	}
	var p UploadProcessPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		c.logger.Error("json.Unmarshal failed", "err", err)
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	log := c.logger.With("upload_id", p.UploadID, "user_id", p.UserID)

	r, err := c.Process(p)
	if err != nil {
		r.Error = err.Error()
	}
	br, err := json.Marshal(r)
	if err != nil {
		log.Error("error serializing processing result", "err", err, "result", r)
		return fmt.Errorf("error serializing processing result: %s (%w)", err, asynq.SkipRetry)
	}

	if r.Error != "" {
		perr := fmt.Errorf("upload processing failed: %s", r.Error)
		if r.Retry {
			log.Warn("upload processing failed", "payload", p, "result", r)
			return perr
		}
		log.Warn("upload processing failed fatally", "payload", p, "result", r)
		_, err = c.resultWriter.Write(br)
		if err != nil {
			log.Error("writing result failed", "err", err)
			perr = fmt.Errorf("%s (also error writing result: %w)", perr, err)
		}
		return fmt.Errorf("%s (%w)", perr, asynq.SkipRetry)
	}

	_, err = c.resultWriter.Write(br)
	if err != nil {
		log.Error("writing result failed", "err", err)
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
	info, err := c.analyzer.Analyze(context.Background(), p.Path, "")
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingAnalyze).Observe(float64(time.Since(t)))
	metrics.AnalysisDuration.Add(float64(time.Since(t)))
	if info == nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingAnalyze).Inc()
		return r, fmt.Errorf("error analyzing file: %w", err)
	}
	log.Debug("stream analyzed", "info", info, "err", err)

	blobPath := path.Join(c.blobsPath, p.UploadID)
	fileName := path.Base(p.Path)
	src := blobs.NewSource(p.Path, blobPath, fileName)

	t = time.Now()
	stream, err := src.Split()
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingBlobSplit).Observe(float64(time.Since(t)))
	if err != nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingBlobSplit).Inc()
		return r, fmt.Errorf("error splitting file: %w", err)
	}
	streamSource := stream.GetSource()
	r.SDHash = hex.EncodeToString(streamSource.GetSdHash())
	defer os.RemoveAll(blobPath)

	t = time.Now()
	summary, err := uploader.Upload(src)
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingReflection).Observe(float64(time.Since(t)))
	metrics.EgressDuration.Add(float64(time.Since(t)))
	metrics.EgressBytes.Add(float64(streamSource.Size))
	metrics.FileSize.WithLabelValues(info.MediaType.Name).Observe(float64(streamSource.Size))
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

	if path.Ext(fileName) == "" {
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

	log.Debug("sending request", "method", p.Request.Method, "patched_fields", patch)
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
	log.Info("stream processed", "method", p.Request.Method)
	return r, nil
}

func (c *Carriage) RetryDelay(n int, err error, t *asynq.Task) time.Duration {
	if errors.Is(err, ErrUpload) {
		return time.Duration(n) * time.Minute
	}
	return 10 * time.Second
}
