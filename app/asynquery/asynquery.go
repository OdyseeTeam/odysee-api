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
	db        boil.Executor
	logger    logging.KVLogger
	belt      *belt.Belt
	respChans map[string]chan<- AsyncQueryResult
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
	Query    AsyncQuery
	Error    string
	Response *jsonrpc.RPCResponse
}

func NewCallManager(redisOpts asynq.RedisConnOpt, logger logging.KVLogger) (*CallManager, error) {
	m := CallManager{
		logger:    logger,
		respChans: make(map[string]chan<- AsyncQueryResult),
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

func (m *CallManager) SetResultChannel(method string, respChan chan<- AsyncQueryResult) {
	m.logger.Debug("adding query result channel", "method", method)
	m.respChans[method] = respChan
}

func (m *CallManager) get(ctx context.Context, id, userID int) (*models.Asynquery, error) {
	l := logging.FromContext(ctx)

	mods := []qm.QueryMod{
		models.AsynqueryWhere.ID.EQ(id),
	}
	if userID > 0 {
		mods = append(mods, models.AsynqueryWhere.UserID.EQ(null.IntFrom(userID)))
	}

	q, err := models.Asynqueries(mods...).One(m.db)
	if err != nil {
		InternalErrors.WithLabelValues("db").Inc()
		l.Warn("could not retrieve asynquery", "err", err)
		return nil, fmt.Errorf("could not retrieve async query record: %w", err)
	}
	return q, nil
}

// Add accepts JSON-RPC request for later asynchronous processing. This may be called from a different process that does Start()
func (m *CallManager) Add(userID int, req *jsonrpc.RPCRequest) error {
	qp := AsyncQuery{
		UserID:  userID,
		Request: req,
	}

	return m.belt.Put(TaskAsyncQuery, qp, 3)
}

func (m *CallManager) initQuery(aq AsyncQuery) (*models.Asynquery, error) {
	q := models.Asynquery{
		UserID: null.IntFrom(aq.UserID),
		Status: models.AsynqueryStatusReceived,
	}
	if err := q.Query.Marshal(aq.Request); err != nil {
		return nil, err
	}
	return &q, q.Insert(m.db, boil.Infer())
}

func (m *CallManager) finishQuery(ctx context.Context, id int, rpcRes *jsonrpc.RPCResponse, callErr string) error {
	l := logging.FromContext(ctx)

	q, err := m.get(ctx, id, 0)
	if err != nil {
		return err
	}

	resp := null.JSON{}
	if err := resp.Marshal(rpcRes); err != nil {
		InternalErrors.WithLabelValues(labelAreaDB).Inc()
		l.Warn("could not marshal rpc response", "err", err)
		return fmt.Errorf("could not marshal rpc response: %w", err)
	}
	q.UpdatedAt = null.TimeFrom(time.Now())
	q.Response = resp
	q.Error = callErr

	if rpcRes.Error != nil || callErr != "" {
		q.Status = models.AsynqueryStatusFailed
	} else {
		q.Status = models.AsynqueryStatusSucceeded
	}

	_, err = q.Update(m.db, boil.Infer())
	if err != nil {
		InternalErrors.WithLabelValues(labelAreaDB).Inc()
		l.Warn("error updating async query record", "err", err)
		return fmt.Errorf("error updating async query record: %w", err)
	}

	return nil
}

// Start launches asynchronous query handlers and blocks until stopped.
func (m *CallManager) Start(db boil.Executor) error {
	m.db = db
	registerServerMetrics()
	m.belt.AddHandler(TaskAsyncQuery, m.HandleTask)
	return m.belt.StartHanders()
}

func (m *CallManager) Shutdown() {
	m.belt.Shutdown()
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
	dbq, err := m.initQuery(aq)
	if err != nil {
		log.Warn("error initializing query", "err", err)
		return err
	}
	aq.QueryID = dbq.ID

	log = log.With("query_id", dbq.ID)

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

	QueriesSent.Inc()
	if err != nil {
		QueriesFailed.Inc()
		log.Warn(sdkNetError.Error(), "err", err)
		if lastTry {
			resErrMsg := err.Error()
			m.finishQuery(ctx, aq.QueryID, nil, err.Error())
			if err != nil {
				log.Warn("failed to finish query processing", "err", err)
				resErrMsg = fmt.Sprintf("%s (also failed to finish query processing: %s)", resErrMsg, err.Error())
			}
			m.sendResult(aq, resErrMsg, nil)
			return sdkNetError
		}
	}

	if res.Error != nil {
		QueriesErrored.Inc()
		log.Info(sdkClientError.Error(), "err", res.Error.Message)
		if lastTry {
			m.finishQuery(ctx, aq.QueryID, res, res.Error.Message)
			if err != nil {
				log.Warn("failed to finish query processing", "err", err)
				return err
			}
			m.sendResult(aq, "", res)
			return sdkClientError
		}
	}
	QueriesCompleted.Inc()
	log.Info("async query completed")
	err = m.finishQuery(ctx, aq.QueryID, res, "")
	if err != nil {
		log.Warn("failed to finish query processing", "err", err)
		return err
	}
	m.sendResult(aq, "", res)

	return nil
}

func (m *CallManager) sendResult(aq AsyncQuery, errorMessage string, res *jsonrpc.RPCResponse) {
	if c, ok := m.respChans[aq.Request.Method]; ok {
		m.logger.Debug("sending query result", "method", aq.Request.Method, "err", errorMessage)
		c <- AsyncQueryResult{
			Query:    aq,
			Error:    errorMessage,
			Response: res,
		}
	}
}

func (m *CallManager) RetryDelay(n int, err error, t *asynq.Task) time.Duration {
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
