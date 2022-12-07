package asynquery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/belt"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/hibiken/asynq"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"
)

const TaskAsyncQuery = "asynquery:query"

var (
	sdkNetError    = errors.New("network-level sdk error")
	sdkClientError = errors.New("client-level sdk error")
)

type CallManager struct {
	db     boil.Executor
	logger logging.KVLogger
	belt   *belt.Belt
}

type Caller struct {
	m      *CallManager
	userID int
}

type AsyncQuery struct {
	UserID  int
	QueryID int
	Request *jsonrpc.RPCRequest
}

type AsyncQueryResult struct {
	QueryID  int
	Response *jsonrpc.RPCRequest
}

type TaskHandler struct {
	results chan<- AsyncQueryResult
	db      boil.Executor
	logger  logging.KVLogger
}

func NewCallManager(db boil.Executor, redisOpts asynq.RedisConnOpt, logger logging.KVLogger) (*CallManager, error) {
	m := CallManager{
		db:     db,
		logger: logger,
	}
	b, err := belt.New(redisOpts, belt.WithConcurrency(10))
	if err != nil {
		return nil, err
	}
	m.belt = b

	return &m, nil
}

func (m *CallManager) NewCaller(userID int) *Caller {
	return &Caller{m: m, userID: userID}
}

func (c *Caller) Call(ctx context.Context, req *jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	return nil, c.m.Add(c.userID, req)
}

func (m *CallManager) Add(userID int, req *jsonrpc.RPCRequest) error {
	query := models.Query{
		UserID: null.IntFrom(userID),
		Status: models.QueryStatusReceived,
	}
	if err := query.Query.Marshal(req); err != nil {
		return err
	}
	err := query.Insert(m.db, boil.Infer())
	if err != nil {
		return err
	}
	qp := AsyncQuery{
		UserID:  userID,
		QueryID: query.ID,
		Request: req,
	}

	return m.belt.Put(TaskAsyncQuery, qp, 3)
}

func (m *CallManager) get(ctx context.Context, id, userID int) (*models.Query, error) {
	l := logging.FromContext(ctx)

	mods := []qm.QueryMod{
		models.QueryWhere.ID.EQ(id),
	}
	if userID > 0 {
		mods = append(mods, models.QueryWhere.UserID.EQ(null.IntFrom(userID)))
	}

	q, err := models.Queries(mods...).One(m.db)
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		l.Warn("could not retrieve asynquery", "err", err)
		return nil, fmt.Errorf("could not retrieve async query record: %w", err)
	}
	return q, nil
}

func (m *CallManager) finish(ctx context.Context, id int, rpcRes *jsonrpc.RPCResponse, callErr string) error {
	l := logging.FromContext(ctx)

	q, err := m.get(ctx, id, 0)
	if err != nil {
		return err
	}

	resp := null.JSON{}
	if err := resp.Marshal(rpcRes); err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		l.Warn("could not marshal rpc response", "err", err)
		return fmt.Errorf("could not marshal rpc response: %w", err)
	}
	q.UpdatedAt = null.TimeFrom(time.Now())
	q.Response = resp
	q.Error = callErr

	if rpcRes.Error != nil || callErr != "" {
		q.Status = models.QueryStatusFailed
	} else {
		q.Status = models.QueryStatusSucceeded
	}

	_, err = q.Update(m.db, boil.Infer())
	if err != nil {
		metrics.Errors.WithLabelValues("db").Inc()
		l.Warn("error updating async query record", "err", err)
		return fmt.Errorf("error updating async query record: %w", err)
	}
	return nil
}

func (m *CallManager) Start() (*belt.Belt, error) {
	m.belt.AddHandler(TaskAsyncQuery, m.HandleTask)
	return m.belt, nil
}

func (m *CallManager) HandleTask(ctx context.Context, task *asynq.Task) error {
	if task.Type() != TaskAsyncQuery {
		m.logger.Warn("cannot handle task", "type", task.Type())
		return asynq.SkipRetry
	}
	var aq AsyncQuery
	if err := json.Unmarshal(task.Payload(), &aq); err != nil {
		m.logger.Warn("message unmarshal failed", "err", err)
		return asynq.SkipRetry
	}

	log := logging.TracedLogger(m.logger, aq)

	u, err := models.Users(
		models.UserWhere.ID.EQ(aq.UserID),
		qm.Load(models.UserRels.LbrynetServer),
	).OneG()
	if err != nil {
		return fmt.Errorf("error getting sdk address for user %v: %w", aq.UserID, err)
	}

	caller := query.NewCaller(u.R.LbrynetServer.Address, aq.UserID)

	log.Info("query sent")
	t := time.Now()
	res, err := caller.Call(context.Background(), aq.Request)
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingQuery).Observe(float64(time.Since(t)))

	mr, _ := asynq.GetMaxRetry(ctx)
	rc, _ := asynq.GetRetryCount(ctx)
	lastTry := mr-rc == 0

	metrics.QueriesSent.Inc()
	if err != nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingQuery).Inc()
		metrics.QueriesFailed.Inc()
		log.Warn(sdkNetError.Error(), "err", err)
		if lastTry {
			m.finish(ctx, aq.QueryID, nil, err.Error())
			return sdkNetError
		}
	}

	if res.Error != nil {
		metrics.ProcessingErrors.WithLabelValues(metrics.LabelProcessingQuery).Inc()
		metrics.QueriesErrored.Inc()
		log.Warn(sdkClientError.Error(), "err", res.Error.Message)
		if lastTry {
			m.finish(ctx, aq.QueryID, res, res.Error.Message)
			return sdkClientError
		}
	}
	metrics.QueriesCompleted.Inc()
	log.Info("async query completed")
	m.finish(ctx, aq.QueryID, res, "")

	return nil
}

func (h *TaskHandler) RetryDelay(n int, err error, t *asynq.Task) time.Duration {
	d := 10 * time.Second
	if errors.Is(err, sdkNetError) {
		return time.Duration(n) * d
	}
	return d
}

func (q AsyncQuery) GetTraceData() map[string]string {
	return map[string]string{
		"user_id": strconv.Itoa(q.UserID),
		"method":  q.Request.Method,
	}
}
