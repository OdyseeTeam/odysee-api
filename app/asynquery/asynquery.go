package asynquery

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/internal/tasks"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/bus"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/hibiken/asynq"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"
)

var (
	sdkNetError    = errors.New("network level sdk error")
	sdkClientError = errors.New("client level sdk error")
	reFilePathURL  = regexp.MustCompile(`^https?://(.+)/(\w+)/\w+/([a-zA-Z0-9]+)$`)
)

type CallManager struct {
	db     boil.Executor
	logger logging.KVLogger
	bus    *bus.Bus
}

type Caller struct {
	manager *CallManager
	userID  int
}

type FileLocation struct {
	Server   string
	Version  string
	UploadID string
}

type Result struct {
	Query    models.Asynquery
	Error    string
	Response *jsonrpc.RPCResponse
}

func NewCallManager(redisOpts asynq.RedisConnOpt, logger logging.KVLogger) (*CallManager, error) {
	m := CallManager{
		logger: logger,
	}
	b, err := bus.New(redisOpts, bus.WithConcurrency(10))
	if err != nil {
		return nil, err
	}
	m.bus = b

	return &m, nil
}

func (m *CallManager) NewCaller(userID int) *Caller {
	return &Caller{manager: m, userID: userID}
}

// Start launches asynchronous query handlers and blocks until stopped.
func (m *CallManager) Start(db boil.Executor) error {
	m.db = db
	registerServerMetrics()
	m.bus.AddHandler(tasks.TaskAsynqueryMerge, m.HandleMerge)
	return m.bus.StartHandlers()
}

func (m *CallManager) Shutdown() {
	m.bus.Shutdown()
}

// Call accepts JSON-RPC request for later asynchronous processing. This may be called from a different process.
func (m *CallManager) Call(userID int, req *jsonrpc.RPCRequest) (*models.Asynquery, error) {
	p := req.Params.(map[string]interface{})
	fp, err := parseFilePath(p["file_path"].(string))
	if err != nil {
		return nil, err
	}

	aq, err := m.createQueryRecord(userID, req, fp.UploadID)
	if err != nil {
		m.logger.Warn("error adding query record", "err", err, "user_id", userID)
		return nil, err
	}

	return aq, nil
}

func (m *CallManager) HandleMerge(ctx context.Context, task *asynq.Task) error {
	if task.Type() != tasks.TaskAsynqueryMerge {
		m.logger.Warn("cannot handle task", "type", task.Type())
		return asynq.SkipRetry
	}
	var payload tasks.AsynqueryMergePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		m.logger.Warn("message unmarshal failed", "err", err)
		return asynq.SkipRetry
	}
	log := logging.TracedLogger(m.logger, payload)

	u, err := models.Users(
		models.UserWhere.ID.EQ(int(payload.UserID)),
		qm.Load(models.UserRels.LbrynetServer),
	).OneG()
	if err != nil {
		return fmt.Errorf("error getting sdk address for user %v: %w", payload.UserID, err)
	}

	aq, err := m.getQueryRecord(context.TODO(), payload.UploadID, payload.UserID)
	if err != nil {
		return err
	}

	caller := query.NewCaller(u.R.LbrynetServer.Address, aq.UserID)

	t := time.Now()

	request := &jsonrpc.RPCRequest{}
	err = aq.Body.Unmarshal(request)
	if err != nil {
		log.Error("failed to unmarshal query body", "err", err)
		return asynq.SkipRetry
	}

	// Patch the original SDK request
	meta := payload.Meta
	patch := map[string]interface{}{
		"file_size": meta.Size,
		"file_name": meta.FileName,
		"file_hash": meta.Hash,
		"sd_hash":   meta.SDHash,
	}
	if meta.Width > 0 && meta.Height > 0 {
		patch["width"] = meta.Width
		patch["height"] = meta.Height
	}
	if meta.Duration > 0 {
		patch["duration"] = meta.Duration
	}

	pp := request.Params.(map[string]interface{})
	for k, v := range patch {
		pp[k] = v
	}
	delete(pp, "file_path")

	res, err := caller.Call(context.TODO(), request)
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
			log.Warn("asynquery failed", "err", resErrMsg)
			m.finalizeQueryRecord(ctx, aq.ID, nil, err.Error())
			if err != nil {
				log.Warn("failed to finalize query record", "err", err)
			}
			return sdkNetError
		}
	}

	if res.Error != nil {
		QueriesErrored.Inc()
		log.Info(sdkClientError.Error(), "err", res.Error.Message)
		if lastTry {
			m.finalizeQueryRecord(ctx, aq.ID, res, res.Error.Message)
			if err != nil {
				log.Warn("failed to finalize query record", "err", err)
				return err
			}
			return sdkClientError
		}
	}
	QueriesCompleted.Inc()
	log.Info("async query completed")
	err = m.finalizeQueryRecord(ctx, aq.ID, res, "")
	if err != nil {
		log.Warn("failed to finalize query record", "err", err)
		return err
	}

	return nil
}

func (m *CallManager) createQueryRecord(userID int, request *jsonrpc.RPCRequest, uploadID string) (*models.Asynquery, error) {
	q := models.Asynquery{
		UserID:   userID,
		UploadID: uploadID,
		Status:   models.AsynqueryStatusReceived,
	}
	q.ID = hash(q)
	if err := q.Body.Marshal(request); err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}
	return &q, q.Insert(m.db, boil.Infer())
}

func (m *CallManager) getQueryRecord(ctx context.Context, uploadID string, userID int32) (*models.Asynquery, error) {
	l := logging.GetFromContext(ctx)

	mods := []qm.QueryMod{
		models.AsynqueryWhere.UploadID.EQ(uploadID),
	}
	if userID > 0 {
		mods = append(mods, models.AsynqueryWhere.UserID.EQ(int(userID)))
	}

	q, err := models.Asynqueries(mods...).One(m.db)
	if err != nil {
		InternalErrors.WithLabelValues("db").Inc()
		l.Warn("could not retrieve asynquery", "err", err)
		return nil, fmt.Errorf("could not retrieve async query record: %w", err)
	}
	return q, nil
}

func (m *CallManager) finalizeQueryRecord(ctx context.Context, queryID string, response *jsonrpc.RPCResponse, callErr string) error {
	l := logging.GetFromContext(ctx)

	q, err := m.getQueryRecord(ctx, queryID, 0)
	if err != nil {
		return err
	}

	jr := null.JSON{}
	if err := jr.Marshal(response); err != nil {
		InternalErrors.WithLabelValues(labelAreaDB).Inc()
		l.Warn("could not marshal rpc response", "err", err)
		return fmt.Errorf("could not marshal rpc response: %w", err)
	}
	q.UpdatedAt = null.TimeFrom(time.Now())
	q.Response = jr
	q.Error = callErr

	if response.Error != nil || callErr != "" {
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

func (m *CallManager) RetryDelay(n int, err error, t *asynq.Task) time.Duration {
	d := 10 * time.Second
	if errors.Is(err, sdkNetError) {
		return time.Duration(n) * d
	}
	return d
}

func parseFilePath(filePath string) (*FileLocation, error) {
	matches := reFilePathURL.FindStringSubmatch(filePath)
	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid file path: %s", filePath)
	}
	fl := &FileLocation{
		Server:   matches[0],
		Version:  matches[1],
		UploadID: matches[2],
	}
	return fl, nil
}

func hash(aq models.Asynquery) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(aq.UploadID)))
}