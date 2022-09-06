package forklift

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lbryio/transcoder/pkg/logging"
	"github.com/lbryio/transcoder/pkg/logging/zapadapter"

	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"
)

const (
	QueueUploadUploadProcessResults = "upload:process:results"
)

type Forklift struct {
	options        *ForkliftOptions
	carriage       *Carriage
	asynqClient    *asynq.Client
	asynqInspector *asynq.Inspector
	asynqServer    *asynq.Server
	stopChan       chan struct{}
	resultBus      *RedisResultBus
}

type ForkliftOptions struct {
	concurrency int
	maxRetry    int
	logger      logging.KVLogger
}

// type ResultBus interface {
// 	Write()
// }

type RedisResultBus struct {
	qname string
	rdb   redis.UniversalClient
}

func WithLogger(logger logging.KVLogger) func(options *ForkliftOptions) {
	return func(options *ForkliftOptions) {
		options.logger = logger
	}
}

func WithConcurrency(concurrency int) func(options *ForkliftOptions) {
	return func(options *ForkliftOptions) {
		options.concurrency = concurrency
	}
}

func WithMaxRetry(maxRetry int) func(options *ForkliftOptions) {
	return func(options *ForkliftOptions) {
		options.maxRetry = maxRetry
	}
}

func NewForklift(blobsPath string, reflectorCfg map[string]string, redisOpts asynq.RedisConnOpt, optionFuncs ...func(*ForkliftOptions)) (*Forklift, error) {
	options := &ForkliftOptions{
		logger:   zapadapter.NewKV(nil),
		maxRetry: 10,
	}
	for _, optionFunc := range optionFuncs {
		optionFunc(options)
	}

	err := redisOpts.MakeRedisClient().(redis.UniversalClient).Ping(context.Background()).Err()
	if err != nil {
		return nil, fmt.Errorf("unable to connect to redis server: %w", err)
	}

	resultBus := NewResultBus(redisOpts, QueueUploadUploadProcessResults)
	c, err := NewCarriage(blobsPath, resultBus, reflectorCfg, options.logger)
	if err != nil {
		return nil, err
	}

	f := &Forklift{
		options:        options,
		carriage:       c,
		asynqClient:    asynq.NewClient(redisOpts),
		asynqInspector: asynq.NewInspector(redisOpts),
		stopChan:       make(chan struct{}),
		resultBus:      resultBus,
	}
	f.asynqServer = asynq.NewServer(
		redisOpts,
		asynq.Config{
			Concurrency: options.concurrency,
			// Optionally specify multiple queues with different priority.
			// Queues: map[string]int{
			// 	"critical": 6,
			// 	"default":  3,
			// 	"low":      1,
			// },
			// Logger:         options.logger,
			RetryDelayFunc: f.RetryDelay,
		},
	)
	return f, nil
}

func NewResultBus(redisOpts asynq.RedisConnOpt, qname string) *RedisResultBus {
	rw := &RedisResultBus{
		qname: qname,
		rdb:   redisOpts.MakeRedisClient().(redis.UniversalClient),
	}
	return rw
}

func (f *Forklift) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeUploadProcess, f.carriage.ProcessTask)
	f.options.logger.Info("starting forklift")
	// if err := f.asynqServer.Run(mux); err != nil {
	// 	return fmt.Errorf("could not run server: %w", err)
	// }
	if err := f.asynqServer.Start(mux); err != nil {
		return fmt.Errorf("could not start asynq server: %w", err)
	}
	return nil
}

func (f *Forklift) Shutdown() {
	f.options.logger.Info("stopping forklift")
	close(f.stopChan)
	f.asynqClient.Close()
	f.asynqServer.Shutdown()
}

func (f *Forklift) EnqueueUploadProcessTask(p UploadProcessPayload) error {
	pb, err := json.Marshal(p)
	if err != nil {
		return err
	}
	f.options.logger.Info("sending upload for processing", "payload", string(pb))
	t := asynq.NewTask(TypeUploadProcess, pb, asynq.MaxRetry(f.options.maxRetry))
	_, err = f.asynqClient.Enqueue(
		t,
		// asynq.Unique(24*time.Hour),
		asynq.Timeout(1*time.Hour),
		asynq.Retention(72*time.Hour),
		// asynq.Queue("critical"),
	)
	return err
}

func (f *Forklift) GetUploadProcessResult() (*UploadProcessResult, error) {
	f.options.logger.Debug("reading upload results")
	d, err := f.resultBus.Read()
	if err != nil {
		return nil, err
	}
	r := &UploadProcessResult{}
	err = json.Unmarshal([]byte(d), r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Forklift) RetryDelay(n int, e error, t *asynq.Task) time.Duration {
	return 5 * time.Second
}

func (w RedisResultBus) Write(data []byte) (int, error) {
	_, err := w.rdb.RPush(context.Background(), w.qname, data).Result()
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func (w *RedisResultBus) Read() (string, error) {
	r, err := w.rdb.BLPop(context.Background(), 0, w.qname).Result()
	if err != nil {
		return "", fmt.Errorf("message reading error: %w", err)
	}
	return r[1], nil
}
