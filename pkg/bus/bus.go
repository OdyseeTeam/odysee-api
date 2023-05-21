// Package bus provides a mechanism for asynchronous task creation and handling between Go services.
// It utilizes the "asynq" package for task queueing and execution.
package bus

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

type BusOptions struct {
	concurrency int
	delayFunc   asynq.RetryDelayFunc
	logger      logging.KVLogger
}

type Bus struct {
	options        *BusOptions
	client         *Client
	asynqInspector *asynq.Inspector
	asynqServer    *asynq.Server
	stopChan       chan struct{}

	handlers map[string]asynq.HandlerFunc
}

type Client struct {
	logger logging.KVLogger
	asynq  *asynq.Client
}

func WithLogger(logger logging.KVLogger) func(options *BusOptions) {
	return func(options *BusOptions) {
		options.logger = logger
	}
}

func WithConcurrency(concurrency int) func(options *BusOptions) {
	return func(options *BusOptions) {
		options.concurrency = concurrency
	}
}

func WithDelayFunc(f asynq.RetryDelayFunc) func(options *BusOptions) {
	return func(options *BusOptions) {
		options.delayFunc = f
	}
}

// New creates a new Bus instance with the provided Redis connection options and optional BusOptions.
// It returns a pointer to the Bus and an error, if any.
func New(redisOpts asynq.RedisConnOpt, optionFuncs ...func(*BusOptions)) (*Bus, error) {
	options := &BusOptions{
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
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	c, err := NewClient(redisOpts, options.logger)
	if err != nil {
		return nil, err
	}

	b := &Bus{
		options:        options,
		client:         c,
		asynqInspector: asynq.NewInspector(redisOpts),
		stopChan:       make(chan struct{}),
		handlers:       map[string]asynq.HandlerFunc{},
	}
	b.asynqServer = asynq.NewServer(
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
	return b, nil
}

// NewClient creates a new Client instance with the provided Redis connection options.
// It returns a pointer to the Client and an error, if any.
func NewClient(redisOpts asynq.RedisConnOpt, logger logging.KVLogger) (*Client, error) {
	err := redisOpts.MakeRedisClient().(redis.UniversalClient).Ping(context.Background()).Err()
	if err != nil {
		return nil, fmt.Errorf("redis client failed: %w", err)
	}

	return &Client{asynq: asynq.NewClient(redisOpts), logger: logger}, nil
}

// AddHandler adds a task handler function for the specified task type.
// Must be called before StartHandlers.
func (b *Bus) AddHandler(taskType string, handler func(context.Context, *asynq.Task) error) {
	b.options.logger.Info("adding task handler", "type", taskType)
	b.handlers[taskType] = handler
}

// StartHandlers launches task handlers and blocks until it's stopped.
func (b *Bus) StartHandlers() error {
	mux := asynq.NewServeMux()
	for k, v := range b.handlers {
		b.options.logger.Info("initializing task handler", "type", k)
		mux.HandleFunc(k, v)
	}
	b.options.logger.Info("starting bus")

	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-t.C:
				q, err := b.asynqInspector.GetQueueInfo("default")
				if err != nil {
					continue
				}
				metrics.QueueTasks.WithLabelValues("active").Set(float64(q.Active))
				metrics.QueueTasks.WithLabelValues("completed").Set(float64(q.Completed))
				metrics.QueueTasks.WithLabelValues("pending").Set(float64(q.Pending))
				metrics.QueueTasks.WithLabelValues("failed").Set(float64(q.Failed))
			case <-b.stopChan:
				return
			}
		}
	}()
	return b.asynqServer.Run(mux)
}

func (b *Bus) Shutdown() {
	b.options.logger.Info("stopping bus")
	close(b.stopChan)
	b.client.Close()
	b.asynqServer.Shutdown()
}

func (b *Bus) Client() *Client {
	return b.client
}

func (c *Client) Put(busName string, payload any, retry int, timeout, retention time.Duration) error {
	if timeout == 0 {
		timeout = 1 * time.Hour
	}
	if retention == 0 {
		retention = 72 * time.Hour
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	t := asynq.NewTask(busName, pb, asynq.MaxRetry(retry))
	c.logger.Debug("adding task", "type", busName, "payload", string(pb))
	_, err = c.asynq.Enqueue(t, asynq.Timeout(timeout), asynq.Retention(retention))
	if err != nil {
		return fmt.Errorf("failed to enqueue task for %s: %w", busName, err)
	}
	return nil
}

func (c *Client) Close() error {
	return c.asynq.Close()
}
