package asynquery

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/geopublish/metrics"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/internal/tasks"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	queue "github.com/OdyseeTeam/odysee-api/pkg/queue"

	"github.com/hibiken/asynq"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc/v2"
)

const FilePathParam = "file_path"

var (
	sdkNetError    = errors.New("network level sdk error")
	sdkClientError = errors.New("client level sdk error")
	reFilePathURL  = regexp.MustCompile(`^https?://([^/]+)/.+/([a-zA-Z0-9\+_\.\-]{32,})$`)

	onceMetrics sync.Once
)

type CallManager struct {
	db     boil.Executor
	logger logging.KVLogger
	queue  *queue.Queue
}

type Caller struct {
	manager *CallManager
	userID  int
}

type FileLocation struct {
	Server   string
	UploadID string
}

type Result struct {
	Query    models.Asynquery
	Error    string
	Response *jsonrpc.RPCResponse
}

type queryParams struct {
	queryID  string
	uploadID string
	userID   int
}

func NewCallManager(redisOpts asynq.RedisConnOpt, db boil.Executor, logger logging.KVLogger) (*CallManager, error) {
	m := CallManager{
		logger: logger,
		db:     db,
	}
	q, err := queue.New(queue.WithRequestsConnOpts(redisOpts), queue.WithConcurrency(10))
	if err != nil {
		return nil, err
	}
	m.queue = q
	logger.Info("asynquery manager created", "concurrency", 10)
	return &m, nil
}

func (m *CallManager) NewCaller(userID int) *Caller {
	return &Caller{manager: m, userID: userID}
}

// Start launches asynchronous query handlers and blocks until stopped.
func (m *CallManager) Start() error {
	onceMetrics.Do(registerMetrics)
	m.queue.AddHandler(tasks.ForkliftUploadDone, m.HandleMerge)
	m.queue.AddHandler(tasks.AsynqueryIncomingQuery, m.HandleQuery)
	return m.queue.ServeUntilShutdown()
}

func (m *CallManager) Shutdown() {
	m.queue.Shutdown()
}

// Call accepts JSON-RPC request for later asynchronous processing.
func (m *CallManager) Call(userID int, req *jsonrpc.RPCRequest) (*models.Asynquery, error) {
	var (
		filePath      string
		filePathParam any
		fileLoc       *FileLocation
		ok            bool
	)
	p := req.Params.(map[string]any)

	// Metadata-only update
	if filePathParam, ok = p[FilePathParam]; !ok {
		aq, err := m.createQueryRecord(queryParams{userID: userID}, req)
		if err != nil {
			m.logger.Warn("error adding query record", "err", err, "user_id", userID)
			return nil, fmt.Errorf("error adding query record: %w", err)
		}
		// Sending query for immediate processing
		err = m.queue.SendRequest(tasks.AsynqueryIncomingQuery, tasks.AsynqueryIncomingQueryPayload{
			QueryID: aq.ID,
			UserID:  userID,
		}, queue.WithRequestRetry(3))
		if err != nil {
			m.logger.Warn("error queing query", "err", err, "user_id", userID)
			return nil, fmt.Errorf("error queuing query: %w", err)
		}
		m.logger.Info("query added and queued", "id", aq.ID, "user_id", userID)
		return aq, nil
	}
	if filePath, ok = filePathParam.(string); !ok {
		return nil, errors.New("invalid file path")
	}
	fileLoc, err := parseFilePath(filePath)
	if err != nil {
		return nil, err
	}

	aq, err := m.getQueryRecord(context.TODO(), queryParams{userID: userID, uploadID: fileLoc.UploadID})
	// If query record exists and is not failed, return it
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil && aq.Status != models.AsynqueryStatusFailed {
		return aq, nil
	}
	aq, err = m.createQueryRecord(queryParams{userID: userID, uploadID: fileLoc.UploadID}, req)
	if err != nil {
		m.logger.Warn("error adding query record", "err", err, "user_id", userID)
		return nil, fmt.Errorf("error adding query record: %w", err)
	}
	m.logger.Info("query added", "id", aq.ID, "user_id", userID, "upload_id", fileLoc.UploadID)

	return aq, nil
}

// HandleMerge handles signals about completed uploads from forklift.
func (m *CallManager) HandleMerge(ctx context.Context, task *asynq.Task) error {
	if task.Type() != tasks.ForkliftUploadDone {
		m.logger.Warn("cannot handle task", "type", task.Type())
		return asynq.SkipRetry
	}
	var payload tasks.ForkliftUploadDonePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		m.logger.Warn("message unmarshal failed", "err", err)
		return asynq.SkipRetry
	}
	log := logging.TracedLogger(m.logger, payload)
	log.Debug("task received")

	aq, err := m.getQueryRecord(ctx, queryParams{uploadID: payload.UploadID, userID: int(payload.UserID)})
	if err != nil {
		log.Info("error getting query record", "err", err)
		return err
	}
	meta := payload.Meta
	patch := map[string]any{
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
	err = m.robustCall(logging.AddToContext(ctx, log), aq, patch)
	if err != nil {
		return err
	}
	return nil
}

// HandleQuery handles asynchronous queries without uploads associated.
func (m *CallManager) HandleQuery(ctx context.Context, task *asynq.Task) error {
	if task.Type() != tasks.AsynqueryIncomingQuery {
		m.logger.Warn("cannot handle task", "type", task.Type())
		return asynq.SkipRetry
	}
	var payload tasks.AsynqueryIncomingQueryPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		m.logger.Warn("message unmarshal failed", "err", err)
		return asynq.SkipRetry
	}
	log := logging.TracedLogger(m.logger, payload)
	log.Debug("task received")

	aq, err := m.getQueryRecord(ctx, queryParams{queryID: payload.QueryID, userID: int(payload.UserID)})
	if err != nil {
		log.Info("error getting query record", "err", err)
		return err
	}
	err = m.robustCall(logging.AddToContext(ctx, log), aq, nil)
	if err != nil {
		return err
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

func (m *CallManager) robustCall(ctx context.Context, aq *models.Asynquery, patch map[string]any) error {
	log := logging.GetFromContext(ctx)
	u, err := models.Users(
		models.UserWhere.ID.EQ(aq.UserID),
		qm.Load(models.UserRels.LbrynetServer),
	).OneG()
	if err != nil {
		log.Info("error getting sdk address for user")
		return asynq.SkipRetry
	}
	sdkAddress := u.R.LbrynetServer.Address

	caller := query.NewCaller(sdkAddress, aq.UserID)

	t := time.Now()

	request := &jsonrpc.RPCRequest{}
	err = aq.Body.Unmarshal(request)
	if err != nil {
		log.Info("failed to unmarshal query body", "err", err)
		return asynq.SkipRetry
	}

	// Patch the original SDK request
	pp, ok := request.Params.(map[string]any)
	if !ok {
		log.Info("cannot extract params from request")
		return asynq.SkipRetry
	}
	if patch != nil {
		for k, v := range patch {
			pp[k] = v
		}
	}
	if request.Method == query.MethodStreamUpdate {
		delete(pp, "name")
		pp["replace"] = true
	}
	delete(pp, "file_path")

	ctx = query.AttachOrigin(ctx, "asynquery")
	res, err := caller.Call(ctx, request)
	metrics.ProcessingTime.WithLabelValues(metrics.LabelProcessingQuery).Observe(float64(time.Since(t)))
	QueriesSent.Inc()

	if err != nil {
		QueriesFailed.Inc()
		log.Warn("asynquery request failed", "err", err)
		monitor.ErrorToSentry(fmt.Errorf("asynquery request failed: %w", err), map[string]string{
			"user_id":  fmt.Sprintf("%d", u.ID),
			"endpoint": sdkAddress,
			"method":   request.Method,
		})

		err := m.finalizeQueryRecord(ctx, aq.ID, nil, err.Error())
		if err != nil {
			log.Warn("failed to finalize asynquery record", "err", err)
		}
		return sdkNetError
	}

	if res.Error != nil {
		QueriesErrored.Inc()
		log.Warn(sdkClientError.Error(), "err", res.Error.Message)
		m.finalizeQueryRecord(ctx, aq.ID, res, res.Error.Message)
		if err != nil {
			log.Warn("failed to finalize query record", "err", err)
			return err
		}
		return asynq.SkipRetry
	}
	QueriesCompleted.Inc()
	log.Info("async query completed", "query_id", aq.ID)
	err = m.finalizeQueryRecord(ctx, aq.ID, res, "")
	if err != nil {
		log.Warn("failed to finalize query record", "err", err)
		return err
	}
	return nil
}

func (m *CallManager) createQueryRecord(params queryParams, request *jsonrpc.RPCRequest) (*models.Asynquery, error) {
	q := models.Asynquery{
		UserID: int(params.userID),
		Status: models.AsynqueryStatusReceived,
	}
	if params.uploadID != "" {
		q.UploadID = params.uploadID
	}
	rb, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}
	q.ID = fmt.Sprintf("%x%v", md5.Sum(rb), time.Now().UnixMilli())
	q.Body = null.JSONFrom(rb)
	return &q, q.Insert(m.db, boil.Infer())
}

func (m *CallManager) getQueryRecord(ctx context.Context, params queryParams) (*models.Asynquery, error) {
	l := logging.GetFromContext(ctx)

	mods := []qm.QueryMod{qm.OrderBy(models.AsynqueryColumns.CreatedAt + " DESC")}
	if params.queryID != "" {
		mods = append(mods, models.AsynqueryWhere.ID.EQ(params.queryID))
	}
	if params.uploadID != "" {
		mods = append(mods, models.AsynqueryWhere.UploadID.EQ(params.uploadID))
	}
	if params.userID > 0 {
		mods = append(mods, models.AsynqueryWhere.UserID.EQ(int(params.userID)))
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

	q, err := m.getQueryRecord(ctx, queryParams{queryID: queryID})
	if err != nil {
		InternalErrors.WithLabelValues("db").Inc()
		return fmt.Errorf("could not retrieve async query record: %w", err)
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

func parseFilePath(filePath string) (*FileLocation, error) {
	matches := reFilePathURL.FindStringSubmatch(filePath)
	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid file location: %s", filePath)
	}
	fl := &FileLocation{
		Server:   matches[1],
		UploadID: matches[2],
	}
	return fl, nil
}
