package belt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"
)

type Belt struct {
	options        *BeltOptions
	asynqClient    *asynq.Client
	asynqInspector *asynq.Inspector
	asynqServer    *asynq.Server
	stopChan       chan struct{}

	handlers map[string]asynq.HandlerFunc
}

type BeltOptions struct {
	concurrency int
	delayFunc   asynq.RetryDelayFunc
	logger      logging.KVLogger
}

func WithLogger(logger logging.KVLogger) func(options *BeltOptions) {
	return func(options *BeltOptions) {
		options.logger = logger
	}
}

func WithConcurrency(concurrency int) func(options *BeltOptions) {
	return func(options *BeltOptions) {
		options.concurrency = concurrency
	}
}

func WithDelayFunc(f asynq.RetryDelayFunc) func(options *BeltOptions) {
	return func(options *BeltOptions) {
		options.delayFunc = f
	}
}

func New(redisOpts asynq.RedisConnOpt, optionFuncs ...func(*BeltOptions)) (*Belt, error) {
	options := &BeltOptions{
		concurrency: 3,
		logger:      zapadapter.NewKV(nil),
		delayFunc: func(n int, err error, t *asynq.Task) time.Duration {
			return 10 * time.Second
		},
	}
	for _, optionFunc := range optionFuncs {
		optionFunc(options)
	}

	err := redisOpts.MakeRedisClient().(redis.UniversalClient).Ping(context.Background()).Err()
	if err != nil {
		return nil, fmt.Errorf("redis client failed: %w", err)
	}

	f := &Belt{
		options:        options,
		asynqClient:    asynq.NewClient(redisOpts),
		asynqInspector: asynq.NewInspector(redisOpts),
		stopChan:       make(chan struct{}),
		handlers:       map[string]asynq.HandlerFunc{},
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
			RetryDelayFunc: options.delayFunc,
		},
	)
	return f, nil
}

func (f *Belt) AddHandler(taskType string, handler func(context.Context, *asynq.Task) error) {
	f.handlers[taskType] = handler
}

// StartHandlers launches task handlers and blocks until it's stopped.
func (f *Belt) StartHanders() error {
	mux := asynq.NewServeMux()
	for k, v := range f.handlers {
		f.options.logger.Info("initializing task handler", "type", k)
		mux.HandleFunc(k, v)
	}
	f.options.logger.Info("starting belt")

	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-t.C:
				q, err := f.asynqInspector.GetQueueInfo("default")
				if err != nil {
					continue
				}
				metrics.QueueTasks.WithLabelValues("active").Set(float64(q.Active))
				metrics.QueueTasks.WithLabelValues("completed").Set(float64(q.Completed))
				metrics.QueueTasks.WithLabelValues("pending").Set(float64(q.Pending))
				metrics.QueueTasks.WithLabelValues("failed").Set(float64(q.Failed))
			case <-f.stopChan:
				return
			}
		}
	}()
	return f.asynqServer.Run(mux)
}

func (f *Belt) Shutdown() {
	f.options.logger.Info("stopping belt")
	close(f.stopChan)
	f.asynqClient.Close()
	f.asynqServer.Shutdown()
}

func (f *Belt) Put(taskType string, payload any, retry int) error {
	pb, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	log := f.options.logger.With("task_type", taskType)
	t := asynq.NewTask(taskType, pb, asynq.MaxRetry(retry))
	_, err = f.asynqClient.Enqueue(t, asynq.Timeout(1*time.Hour), asynq.Retention(72*time.Hour))
	if err != nil {
		log.Warn("failed to put task", "err", err)
	} else {
		log.Debug("task put")
	}
	return err
}
