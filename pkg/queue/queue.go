// Package queue provides a request-response mechanism for asynchronous message passing between separate services.
// It works best for scenarios where processing requires execution guarantees but is time-consuming and/or failure-prone.
// It utilizes the "asynq" package for message delivery.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type Options struct {
	concurrency       int
	delayFunc         asynq.RetryDelayFunc
	logger            logging.KVLogger
	requestsConnOpts  asynq.RedisConnOpt
	requestsConnURL   string
	responsesConnOpts asynq.RedisConnOpt
	responsesConnURL  string
}

type MessageOptions struct {
	retry              int
	timeout, retention time.Duration
}

type Queue struct {
	options         *Options
	requestsClient  *asynq.Client
	responsesClient *asynq.Client
	asynqInspector  *asynq.Inspector
	asynqServer     *asynq.Server
	handlerStopChan chan struct{}
	handlers        map[string]asynq.HandlerFunc
	logger          logging.KVLogger
}

func WithRequestsConnOpts(opts asynq.RedisConnOpt) func(options *Options) {
	return func(options *Options) {
		options.requestsConnOpts = opts
	}
}

func WithResponsesConnOpts(opts asynq.RedisConnOpt) func(options *Options) {
	return func(options *Options) {
		options.responsesConnOpts = opts
	}
}

func WithRequestsConnURL(url string) func(options *Options) {
	return func(options *Options) {
		opts, err := asynq.ParseRedisURI(url)
		if err != nil {
			panic(err)
		}
		options.requestsConnOpts = opts
		options.requestsConnURL = url
	}
}

func WithResponsesConnURL(url string) func(options *Options) {
	return func(options *Options) {
		opts, err := asynq.ParseRedisURI(url)
		if err != nil {
			panic(err)
		}
		options.responsesConnOpts = opts
		options.responsesConnURL = url
	}
}

func WithConcurrency(concurrency int) func(options *Options) {
	return func(options *Options) {
		options.concurrency = concurrency
	}
}

func WithDelayFunc(f asynq.RetryDelayFunc) func(options *Options) {
	return func(options *Options) {
		options.delayFunc = f
	}
}

func WithLogger(logger logging.KVLogger) func(options *Options) {
	return func(options *Options) {
		options.logger = logger
	}
}

func WithRequestRetry(retry int) func(options *MessageOptions) {
	return func(options *MessageOptions) {
		options.retry = retry
	}
}

func WithRequestTimeout(timeout time.Duration) func(options *MessageOptions) {
	return func(options *MessageOptions) {
		options.timeout = timeout
	}
}

func WithRequestRetention(retention time.Duration) func(options *MessageOptions) {
	return func(options *MessageOptions) {
		options.retention = retention
	}
}

// New creates a new Queue instance with Redis request and response connections.
// If supplied WithRequestsConnOpts, the handler will be able to receive requests.
// If supplied WithResponsesConnOpts, the handler will be able to send responses.
// Response connection is provided for convenience, there is no coordination mechanism,
// each response is just another independent request sent to another queue.
func New(optionFuncs ...func(*Options)) (*Queue, error) {
	options := &Options{
		concurrency: 3,
		logger:      zapadapter.NewKV(nil),
		delayFunc: func(n int, err error, t *asynq.Task) time.Duration {
			return 10 * time.Second
		},
	}
	for _, optionFunc := range optionFuncs {
		optionFunc(options)
	}

	var err error
	var conn bool
	queue := &Queue{
		options:         options,
		handlerStopChan: make(chan struct{}),
		handlers:        map[string]asynq.HandlerFunc{},
		logger:          options.logger,
	}

	if options.responsesConnOpts != nil {
		err = options.responsesConnOpts.MakeRedisClient().(redis.UniversalClient).Ping(context.Background()).Err()
		if err != nil {
			return nil, fmt.Errorf("redis responses connection failed: %w", err)
		}
		queue.responsesClient = asynq.NewClient(options.responsesConnOpts)
		conn = true
	}
	if options.requestsConnOpts != nil {
		err = options.requestsConnOpts.MakeRedisClient().(redis.UniversalClient).Ping(context.Background()).Err()
		if err != nil {
			return nil, fmt.Errorf("redis requests connection failed: %w", err)
		}
		queue.asynqInspector = asynq.NewInspector(options.requestsConnOpts)
		queue.requestsClient = asynq.NewClient(options.requestsConnOpts)
		conn = true
	}
	if !conn {
		return nil, errors.New("either requests or responses connection options must be provided")
	}

	return queue, nil
}

// NewWithResponses creates a new Queue instance with mandatory request and response connection URLs.
func NewWithResponses(requestsConnURL, responsesConnURL string, optionFuncs ...func(*Options)) (*Queue, error) {
	if requestsConnURL == "" || responsesConnURL == "" {
		return nil, errors.New("both requests and responses connection URL must be provided")
	}
	optionFuncs = append(optionFuncs, WithRequestsConnURL(requestsConnURL), WithResponsesConnURL(responsesConnURL))
	return New(optionFuncs...)
}

// AddHandler adds a request handler function for the specified request type.
// Must be called before ServeUntilShutdown.
func (q *Queue) AddHandler(requestType string, handler func(context.Context, *asynq.Task) error) {
	q.logger.Info("adding request handler", "type", requestType)
	q.handlers[requestType] = handler
}

// ServeUntilShutdown launches request handlers and blocks until it's stopped.
func (q *Queue) ServeUntilShutdown() error {
	if q.options.requestsConnOpts == nil {
		return errors.New("requests connection options must be provided")
	}
	if len(q.handlers) == 0 {
		return errors.New("no request handlers registered")
	}
	q.asynqServer = asynq.NewServer(
		q.options.requestsConnOpts,
		asynq.Config{
			Concurrency:    q.options.concurrency,
			RetryDelayFunc: q.options.delayFunc,
			Logger:         zapadapter.New(nil),
		},
	)
	mux := asynq.NewServeMux()
	for k, v := range q.handlers {
		q.logger.Info("initializing request handler", "type", k)
		mux.HandleFunc(k, v)
	}
	q.logger.Info("started queue handlers")

	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-t.C:
				q, err := q.asynqInspector.GetQueueInfo("default")
				if err != nil {
					continue
				}
				queueTasks.WithLabelValues("active").Set(float64(q.Active))
				queueTasks.WithLabelValues("completed").Set(float64(q.Completed))
				queueTasks.WithLabelValues("pending").Set(float64(q.Pending))
				queueTasks.WithLabelValues("failed").Set(float64(q.Failed))
			case <-q.handlerStopChan:
				return
			}
		}
	}()
	err := q.asynqServer.Run(mux)
	if err != nil {
		q.logger.Error("error starting queue", "error", err)
	}
	return err
}

func (q *Queue) Shutdown() {
	q.logger.Info("stopping queue")
	close(q.handlerStopChan)
	if q.responsesClient != nil {
		q.responsesClient.Close()
	}
	if q.requestsClient != nil {
		q.requestsClient.Close()
	}
	if q.asynqInspector != nil {
		q.asynqInspector.Close()
	}
	if q.asynqServer != nil {
		q.asynqServer.Shutdown()
	}
}

func (q *Queue) SendResponse(responseType string, payload any, optionFuncs ...func(*MessageOptions)) error {
	if q.responsesClient == nil {
		return errors.New("response client is missing")
	}
	return q.sendMessage(q.responsesClient, responseType, payload, optionFuncs...)
}

func (q *Queue) SendRequest(requestType string, payload any, optionFuncs ...func(*MessageOptions)) error {
	if q.requestsClient == nil {
		return errors.New("requests client is missing")
	}
	return q.sendMessage(q.requestsClient, requestType, payload, optionFuncs...)

}

func (q *Queue) sendMessage(client *asynq.Client, messageType string, payload any, optionFuncs ...func(*MessageOptions)) error {
	options := &MessageOptions{
		retry:     3,
		timeout:   1 * time.Hour,
		retention: 72 * time.Hour,
	}
	for _, optionFunc := range optionFuncs {
		optionFunc(options)
	}

	pb, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	t := asynq.NewTask(messageType, pb, asynq.MaxRetry(options.retry))
	q.logger.Debug(
		"sending message", "type", messageType,
		"payload", string(pb), "retries", options.retry,
		"timeout", options.timeout, "retention", options.retention)
	_, err = client.Enqueue(t, asynq.Timeout(options.timeout), asynq.Retention(options.retention))
	if err != nil {
		return fmt.Errorf("failed to enqueue %s request: %w", messageType, err)
	}
	return nil
}
