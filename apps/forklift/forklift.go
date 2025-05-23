package forklift

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/uploads/database"
	"github.com/OdyseeTeam/odysee-api/internal/tasks"
	"github.com/OdyseeTeam/odysee-api/pkg/blobs"
	"github.com/OdyseeTeam/odysee-api/pkg/fileanalyzer"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/queue"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/spf13/viper"
	"github.com/sqlc-dev/pqtype"
)

var ErrReflector = errors.New("errors found while uploading blobs to reflector")

type Deleter interface {
	Delete(context.Context, tasks.FileLocationS3) error
}

type Launcher struct {
	blobPath         string
	downloadsPath    string
	requestsConnURL  string
	responsesConnURL string
	reflectorConfig  *viper.Viper
	retriever        *S3Retriever
	httpRetriever    *HTTPRetriever
	logger           logging.KVLogger
	db               database.DBTX
	concurrency      int
	reflectorWorkers int
	metricsAddress   string
	s3client         *s3.Client
	forklift         *Forklift
}

type Forklift struct {
	analyzer      *fileanalyzer.Analyzer
	blobPath      string
	uploadPath    string
	logger        logging.KVLogger
	retriever     *S3Retriever
	httpRetriever *HTTPRetriever
	store         *blobs.Store
	queries       *database.Queries
	queue         *queue.Queue
}

type LauncherOption func(l *Launcher)

func WithLogger(logger logging.KVLogger) LauncherOption {
	return func(l *Launcher) {
		l.logger = logger
	}
}

func WithDownloadsPath(path string) LauncherOption {
	return func(l *Launcher) {
		l.downloadsPath = path
	}
}

func WithBlobPath(path string) LauncherOption {
	return func(l *Launcher) {
		l.blobPath = path
	}
}

func WithRequestsConnURL(url string) LauncherOption {
	return func(l *Launcher) {
		l.requestsConnURL = url
	}
}

func WithResponsesConnURL(url string) LauncherOption {
	return func(l *Launcher) {
		l.responsesConnURL = url
	}
}

func WithReflectorConfig(cfg *viper.Viper) LauncherOption {
	return func(l *Launcher) {
		l.reflectorConfig = cfg
	}
}

func WithS3Client(client *s3.Client) LauncherOption {
	return func(l *Launcher) {
		l.s3client = client
	}
}

func WithRetriever(retriever *S3Retriever) LauncherOption {
	return func(l *Launcher) {
		l.retriever = retriever
	}
}

func WithHTTPRetriever(retriever *HTTPRetriever) LauncherOption {
	return func(l *Launcher) {
		l.httpRetriever = retriever
	}
}

func WithConcurrency(concurrency int) LauncherOption {
	return func(l *Launcher) {
		l.concurrency = concurrency
	}
}

// WithReflectorWorkers sets the number of workers uploading each stream to the reflector.
func WithReflectorWorkers(workers int) LauncherOption {
	return func(l *Launcher) {
		l.reflectorWorkers = workers
	}
}

func WithDB(db database.DBTX) LauncherOption {
	return func(l *Launcher) {
		l.db = db
	}
}

func ExposeMetrics(address ...string) LauncherOption {
	return func(l *Launcher) {
		if len(address) == 0 {
			l.metricsAddress = ":8080"
		} else {
			l.metricsAddress = address[0]
		}
	}
}

func NewLauncher(options ...LauncherOption) *Launcher {
	launcher := &Launcher{
		logger:           logging.NoopKVLogger{},
		concurrency:      10,
		reflectorWorkers: 1,
	}

	for _, option := range options {
		option(launcher)
	}

	var err error
	if launcher.blobPath == "" {
		launcher.blobPath, err = os.MkdirTemp("", "forklift-blobs")
		if err != nil {
			panic(err)
		}
	}
	if launcher.downloadsPath == "" {
		launcher.downloadsPath, err = os.MkdirTemp("", "forklift-downloads")
		if err != nil {
			panic(err)
		}
	}
	if launcher.httpRetriever == nil {
		launcher.httpRetriever = NewHTTPRetriever(launcher.downloadsPath)
	}
	if launcher.retriever == nil {
		launcher.retriever = NewS3Retriever(launcher.downloadsPath, launcher.s3client)
	}
	return launcher
}

func (l *Launcher) Build() (*queue.Queue, error) {
	if l.db == nil {
		return nil, errors.New("database is required")
	}
	if l.retriever == nil {
		return nil, errors.New("retriever is required")
	}
	if l.httpRetriever == nil {
		l.httpRetriever = NewHTTPRetriever(l.downloadsPath)
	}

	analyzer, err := fileanalyzer.NewAnalyzer()
	if err != nil {
		return nil, err
	}

	destinations, err := blobs.CreateStoresFromConfig(l.reflectorConfig, "destinations")
	if err != nil {
		return nil, fmt.Errorf("cannot initialize reflector destination stores: %w", err)
	}
	store, err := blobs.NewStore(l.reflectorConfig.GetString("databasedsn"), destinations)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize reflector store: %w", err)
	}
	store.SetWorkers(l.reflectorWorkers)
	l.logger.Info("reflector store initialized")

	taskQueue, err := queue.NewWithResponses(
		l.requestsConnURL, l.responsesConnURL,
		queue.WithConcurrency(l.concurrency),
		queue.WithLogger(l.logger))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize queue: %w", err)
	}

	if l.metricsAddress != "" {
		listener, err := net.Listen("tcp", ":8080") // adjust arguments as needed.
		if err != nil {
			return nil, fmt.Errorf("failed to bind http metrics server to %s: %w", l.metricsAddress, err)
		}
		router := chi.NewRouter()
		router.Handle("/internal/metrics", BuildMetricsHandler())
		httpServer := &http.Server{
			Addr:    l.metricsAddress,
			Handler: router,
		}

		go func() {
			if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
				l.logger.Error("http server returned error", "err", err)
			}
		}()
	}
	l.logger.Info("metrics server launched", "addr", l.metricsAddress)

	forklift := &Forklift{
		analyzer:      analyzer,
		blobPath:      l.blobPath,
		logger:        l.logger,
		retriever:     l.retriever,
		httpRetriever: l.httpRetriever,
		store:         store,
		queries:       database.New(l.db),
		queue:         taskQueue,
	}
	l.forklift = forklift
	taskQueue.AddHandler(tasks.ForkliftUploadIncoming, forklift.HandleUpload)
	taskQueue.AddHandler(tasks.ForkliftURLIncoming, forklift.HandleURL)
	l.logger.Info("forklift initialized")
	return taskQueue, nil
}

func (f *Forklift) HandleUpload(ctx context.Context, task *asynq.Task) error {
	if task.Type() != tasks.ForkliftUploadIncoming {
		f.logger.Warn("cannot handle task", "type", task.Type())
		return asynq.SkipRetry
	}
	start := time.Now()
	waitStart := time.Now()
	defer func() {
		waitTimeMinutes.Observe(time.Since(waitStart).Minutes())
	}()

	var payload tasks.ForkliftUploadIncomingPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		f.logger.Warn("message unmarshal failed", "err", err)
		return asynq.SkipRetry
	}

	log := logging.TracedLogger(f.logger, payload)
	log.Debug("task received")

	localFile, err := f.retriever.Retrieve(context.TODO(), payload.UploadID, payload.FileLocation)
	if err != nil {
		log.Warn("failed to retrieve file", "err", err)
		return err
	}
	defer localFile.Cleanup()
	observeDuration(LabelRetrieve, start)
	log.Debug("file retrieved", "location", payload.FileLocation, "size", localFile.Size, "seconds", time.Since(start).Seconds())

	blobPath := path.Join(f.blobPath, localFile.Name)
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
	info, err := f.analyzer.Analyze(context.Background(), localFile.Name, payload.FileName)
	observeDuration(LabelAnalyze, start)
	if info == nil {
		observeError(LabelAnalyze)
		log.Warn("file analysis failed", "err", err, "file", localFile.Name)
		return err
	}
	log.Debug("file analyzed", "result", info, "err", err)

	src := blobs.NewSource(localFile.Name, blobPath, payload.FileName)
	start = time.Now()
	log.Debug("creating stream")
	stream, err := src.Split()
	observeDuration(LabelStreamCreate, start)
	if err != nil {
		observeError(LabelStreamCreate)
		log.Warn("failed to create stream", "err", err, "file", localFile.Name, "blobs_path", f.blobPath)
		return err
	}
	streamSource := stream.GetSource()
	sdHash := hex.EncodeToString(streamSource.GetSdHash())

	log = log.With("sd_hash", sdHash)
	log.Debug("stream created", "seconds", time.Since(start).Seconds())

	start = time.Now()
	log.Debug("starting upload")
	summary, err := uploader.Upload(src)
	observeDuration(LabelUpstream, start)
	egressDurationSeconds.Add(float64(time.Since(start)))
	egressVolumeMB.Add(float64(streamSource.GetSize() / 1024 / 1024))
	if err != nil {
		observeError(LabelUpstream)
		log.Warn("blobs upload failed, not retrying", "err", err, "blobs_path", f.blobPath)
		return err
	} else if summary.Err > 0 {
		observeError(LabelUpstream)
		log.Warn(ErrReflector.Error(), "err_count", summary.Err, "blobs_path", f.blobPath)
		return ErrReflector
	}
	log.Debug("stream blobs uploaded", "seconds", time.Since(start).Seconds())

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

	defer func() {
		err := f.retriever.Delete(context.TODO(), payload.FileLocation)
		if err != nil {
			log.Warn("failed to complete retrieved file", "err", err)
		}
	}()

	err = f.queue.SendResponse(tasks.ForkliftUploadDone, tasks.ForkliftUploadDonePayload{
		UploadID: payload.UploadID,
		UserID:   payload.UserID,
		Meta:     meta,
	}, queue.WithRequestRetry(15), queue.WithRequestTimeout(15*time.Minute))
	if err != nil {
		log.Error("merge request failed, bus error", "err", err)
		return err
	}
	log.Debug("forklift done")

	return nil
}

func (f *Forklift) HandleURL(ctx context.Context, task *asynq.Task) error {
	if task.Type() != tasks.ForkliftURLIncoming {
		f.logger.Warn("cannot handle task", "type", task.Type())
		return asynq.SkipRetry
	}
	start := time.Now()
	waitStart := time.Now()
	defer func() {
		waitTimeMinutes.Observe(time.Since(waitStart).Minutes())
	}()

	var payload tasks.ForkliftURLIncomingPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		f.logger.Warn("message unmarshal failed", "err", err)
		return asynq.SkipRetry
	}

	log := logging.TracedLogger(f.logger, payload)
	log.Debug("task received")

	localFile, err := f.httpRetriever.Retrieve(context.TODO(), payload.UploadID, payload.FileLocation)
	if err != nil {
		log.Info("failed to retrieve file", "err", err)
		return err
	}
	defer localFile.Cleanup()
	observeDuration(LabelRetrieve, start)
	log.Debug("file retrieved", "location", payload.FileLocation, "size", localFile.Size, "seconds", time.Since(start).Seconds())

	blobPath := path.Join(f.blobPath, localFile.Name)
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
	info, err := f.analyzer.Analyze(context.Background(), localFile.Name, payload.FileName)
	observeDuration(LabelAnalyze, start)
	if info == nil {
		observeError(LabelAnalyze)
		log.Warn("file analysis failed", "err", err, "file", localFile.Name)
		return err
	}
	log.Debug("file analyzed", "result", info, "err", err)

	src := blobs.NewSource(localFile.Name, blobPath, payload.FileName)
	start = time.Now()
	log.Debug("creating stream")
	stream, err := src.Split()
	observeDuration(LabelStreamCreate, start)
	if err != nil {
		observeError(LabelStreamCreate)
		log.Warn("failed to create stream", "err", err, "file", localFile.Name, "blobs_path", f.blobPath)
		return err
	}
	streamSource := stream.GetSource()
	sdHash := hex.EncodeToString(streamSource.GetSdHash())

	log = log.With("sd_hash", sdHash)
	log.Debug("stream created", "seconds", time.Since(start).Seconds())

	start = time.Now()
	log.Debug("starting upload")
	summary, err := uploader.Upload(src)
	observeDuration(LabelUpstream, start)
	egressDurationSeconds.Add(float64(time.Since(start)))
	egressVolumeMB.Add(float64(streamSource.GetSize() / 1024 / 1024))
	if err != nil {
		observeError(LabelUpstream)
		log.Warn("blobs upload failed, not retrying", "err", err, "blobs_path", f.blobPath)
		return err
	} else if summary.Err > 0 {
		observeError(LabelUpstream)
		log.Warn(ErrReflector.Error(), "err_count", summary.Err, "blobs_path", f.blobPath)
		return ErrReflector
	}
	log.Debug("stream blobs uploaded", "seconds", time.Since(start).Seconds())

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

	err = f.queue.SendResponse(tasks.ForkliftUploadDone, tasks.ForkliftUploadDonePayload{
		UploadID: payload.UploadID,
		UserID:   payload.UserID,
		Meta:     meta,
	}, queue.WithRequestRetry(15), queue.WithRequestTimeout(15*time.Minute))
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
