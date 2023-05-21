package forklift

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/uploads/database"
	"github.com/OdyseeTeam/odysee-api/internal/tasks"
	"github.com/OdyseeTeam/odysee-api/pkg/blobs"
	"github.com/OdyseeTeam/odysee-api/pkg/bus"
	"github.com/OdyseeTeam/odysee-api/pkg/fileanalyzer"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/tabbed/pqtype"

	"github.com/hibiken/asynq"
)

var ErrReflector = errors.New("errors found while uploading blobs to reflector")

type Launcher struct {
	blobPath        string
	redisURL        string
	reflectorConfig map[string]string
	retriever       Retriever
	logger          logging.KVLogger
	db              database.DBTX
	concurrency     int
}

type Forklift struct {
	analyzer  *fileanalyzer.Analyzer
	blobPath  string
	bus       *bus.Bus
	logger    logging.KVLogger
	retriever Retriever
	store     *blobs.Store
	queries   *database.Queries
}

type LauncherOption func(l *Launcher)

func WithLogger(logger logging.KVLogger) LauncherOption {
	return func(l *Launcher) {
		l.logger = logger
	}
}

func WithBlobPath(blobPath string) LauncherOption {
	return func(l *Launcher) {
		l.blobPath = blobPath
	}
}

func WithRedisURL(redisURL string) LauncherOption {
	return func(l *Launcher) {
		l.redisURL = redisURL
	}
}

func WithReflectorConfig(reflectorConfig map[string]string) LauncherOption {
	return func(l *Launcher) {
		l.reflectorConfig = reflectorConfig
	}
}

func WithRetriever(retreiver Retriever) LauncherOption {
	return func(l *Launcher) {
		l.retriever = retreiver
	}
}

func WithConcurrency(concurrency int) LauncherOption {
	return func(l *Launcher) {
		l.concurrency = concurrency
	}
}

func WithDB(db database.DBTX) LauncherOption {
	return func(l *Launcher) {
		l.db = db
	}
}
func NewLauncher(options ...LauncherOption) *Launcher {
	launcher := &Launcher{
		logger:      logging.NoopKVLogger{},
		blobPath:    os.TempDir(),
		concurrency: 10,
	}

	for _, option := range options {
		option(launcher)
	}

	return launcher
}

func (l *Launcher) Build() (*bus.Bus, error) {
	redisOpts, err := asynq.ParseRedisURI(l.redisURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to redis: %w", err)
	}

	if l.db == nil {
		return nil, errors.New("database is required")
	}
	if l.retriever == nil {
		return nil, errors.New("retriever is required")
	}

	analyzer, err := fileanalyzer.NewAnalyzer()
	if err != nil {
		return nil, err
	}

	s, err := blobs.NewStore(l.reflectorConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize reflector store: %w", err)
	}

	f := &Forklift{
		analyzer:  analyzer,
		blobPath:  l.blobPath,
		logger:    l.logger,
		retriever: l.retriever,
		store:     s,
		queries:   database.New(l.db),
	}

	b, err := bus.New(redisOpts, bus.WithLogger(l.logger), bus.WithConcurrency(l.concurrency))
	if err != nil {
		return nil, fmt.Errorf("unable to start task bus: %w", err)
	}
	b.AddHandler(tasks.TaskReflectUpload, f.HandleTask)
	f.bus = b

	return b, nil
}

func (f *Forklift) HandleTask(ctx context.Context, task *asynq.Task) error {
	if task.Type() != tasks.TaskReflectUpload {
		f.logger.Warn("cannot handle task", "type", task.Type())
		return asynq.SkipRetry
	}
	var payload tasks.ReflectUploadPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		f.logger.Warn("message unmarshal failed", "err", err)
		return asynq.SkipRetry
	}

	log := logging.TracedLogger(f.logger, payload)
	start := time.Now()
	file, err := f.retriever.Retrieve(context.TODO(), payload.UploadID, payload.FileLocation)
	if err != nil {
		log.Warn("failed to retrieve file", "err", err)
		return asynq.SkipRetry
	}
	defer file.Cleanup()
	observeDuration(LabelRetrieve, start)
	log.Debug("file retreived", "location", payload.FileLocation, "size", file.Size, "seconds", time.Since(start).Seconds())

	blobPath := path.Join(f.blobPath, payload.UploadID)
	if err != nil {
		log.Warn("failed to close upload file", "err", err)
	}
	defer func() {
		if err := os.RemoveAll(blobPath); err != nil {
			log.Warn("failed to remove blobs", "err", err)
		}
	}()

	uploader := f.store.Uploader()

	start = time.Now()
	info, err := f.analyzer.Analyze(context.Background(), file.Name, payload.FileName)
	observeDuration(LabelAnalyze, start)
	if info == nil {
		observeError(LabelAnalyze)
		log.Warn("file analysis failed", "err", err, "file", file.Name)
		return err
	}
	log.Debug("file analyzed", "result", info, "err", err)

	src := blobs.NewSource(file.Name, blobPath)
	start = time.Now()
	stream, err := src.Split()
	observeDuration(LabelSplit, start)
	if err != nil {
		observeError(LabelSplit)
		log.Warn("failed to split stream", "err", err, "file", file.Name, "blobs_path", f.blobPath)
		return err
	}
	streamSource := stream.GetSource()
	sdHash := hex.EncodeToString(streamSource.GetSdHash())

	log = log.With("sd_hash", sdHash)
	log.Debug("file split", "seconds", time.Since(start).Seconds())

	start = time.Now()
	summary, err := uploader.Upload(src)
	observeDuration(LabelUpstream, start)
	if err != nil {
		// With errors returned by the current implementation of uploader it doesn't make sense to retry.
		observeError(LabelUpstream)
		log.Warn("blobs upload failed, not retrying", "err", err, "blobs_path", f.blobPath)
		return asynq.SkipRetry
	} else if summary.Err > 0 {
		observeError(LabelUpstream)
		log.Warn(ErrReflector.Error(), "err_count", summary.Err, "blobs_path", f.blobPath)
		return ErrReflector
	}
	log.Debug("blobs uploaded", "seconds", time.Since(start).Seconds())

	meta := tasks.UploadMeta{
		Hash:      hex.EncodeToString(streamSource.GetHash()),
		MIME:      info.MediaType.MIME,
		FileName:  payload.FileName,
		Extension: info.MediaType.Extension,
		Size:      streamSource.Size,
		SDHash:    sdHash,
	}

	if path.Ext(meta.FileName) == "" {
		meta.FileName += info.MediaType.Extension
	}

	if info.MediaInfo != nil {
		meta.Width = info.MediaInfo.Width
		meta.Height = info.MediaInfo.Height
		meta.Duration = info.MediaInfo.Duration
	}

	jbMeta, err := json.Marshal(meta)
	if err != nil {
		log.Error("failed to marshal media info", "err", err)
	}

	err = f.queries.MarkUploadProcessed(context.TODO(), database.MarkUploadProcessedParams{
		ID:     payload.UploadID,
		SDHash: sdHash,
		Meta:   pqtype.NullRawMessage{RawMessage: jbMeta, Valid: true},
	})
	if err != nil {
		log.Error("failed to mark upload as processed", "err", err)
		return err
	}
	log.Debug("upload processed")

	err = f.bus.Client().Put(tasks.TaskAsynqueryMerge, tasks.AsynqueryMergePayload{
		UploadID: payload.UploadID,
		UserID:   payload.UserID,
		Meta:     meta,
	}, 99, 15*time.Minute, 72*time.Hour)
	if err != nil {
		log.Error("merge request failed, bus error", "err", err)
		return err
	}
	log.Debug("forklift done")

	return nil
}

func (c *Forklift) RetryDelay(count int, err error, t *asynq.Task) time.Duration {
	if errors.Is(err, ErrReflector) {
		return time.Duration(count) * time.Minute
	}
	return 10 * time.Second
}
