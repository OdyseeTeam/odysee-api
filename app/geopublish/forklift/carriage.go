package forklift

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/blobs"
	"github.com/OdyseeTeam/odysee-api/pkg/fileanalyzer"
	"github.com/lbryio/transcoder/pkg/logging"
	"github.com/lbryio/transcoder/pkg/logging/zapadapter"

	"github.com/hibiken/asynq"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/ybbus/jsonrpc"
)

// A list of task types.
const (
	TypeUploadProcess = "upload:process"
)

type UploadProcessPayload struct {
	UploadID string
	Path     string
	UserID   int
	Request  *jsonrpc.RPCRequest
}

type UploadProcessResult struct {
	UploadID string
	UserID   int
	SDHash   string
	Response *jsonrpc.RPCResponse
	Error    string
	Retry    bool
}

type Carriage struct {
	blobsPath    string
	analyzer     *fileanalyzer.Analyzer
	reflectorCfg map[string]string
	resultWriter io.Writer
	logger       logging.KVLogger
}

func NewCarriage(blobsPath string, resultWriter io.Writer, reflectorCfg map[string]string, logger logging.KVLogger) (*Carriage, error) {
	analyzer, err := fileanalyzer.NewAnalyzer()
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = zapadapter.NewKV(nil)
	}

	c := &Carriage{
		reflectorCfg: reflectorCfg,
		analyzer:     analyzer,
		blobsPath:    blobsPath,
		resultWriter: resultWriter,
		logger:       logger,
	}
	return c, nil
}

func (c *Carriage) ProcessTask(ctx context.Context, t *asynq.Task) error {
	if t.Type() != TypeUploadProcess {
		c.logger.Warn("cannot handle task", "type", t.Type())
		return fmt.Errorf("cannot handle %s", t.Type())
	}
	var p UploadProcessPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	r, err := c.Process(p)
	if err != nil {
		r.Error = err.Error()
	}
	br, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("error serializing processing result: %s (%w)", err, asynq.SkipRetry)
	}

	if err != nil {
		c.logger.Warn("upload processing failed", "result", r, "payload", p)
		perr := fmt.Errorf("upload processing failed: %s", r.Error)
		if r.Retry {
			return perr
		}
		_, err = c.resultWriter.Write(br)
		if err != nil {
			perr = fmt.Errorf("%s (also error writing result: %w)", perr, err)
		}
		return fmt.Errorf("%s (%w)", perr, asynq.SkipRetry)
	}

	_, err = c.resultWriter.Write(br)
	if err != nil {
		return fmt.Errorf("error writing result: %w", err)
	}
	return nil
}

func (c *Carriage) Process(p UploadProcessPayload) (*UploadProcessResult, error) {
	r := &UploadProcessResult{UploadID: p.UploadID, UserID: p.UserID}
	uploader, err := blobs.NewUploaderFromCfg(c.reflectorCfg)
	if err != nil {
		return nil, err
	}

	info, err := c.analyzer.Analyze(context.Background(), p.Path)
	if info == nil {
		return r, err
	} // else warn about error

	src, err := blobs.NewSource(p.Path, c.blobsPath)
	if err != nil {
		return r, err
	}

	stream, err := src.Split()
	// TODO: Retry upload on failure
	if err != nil {
		return r, err
	}
	streamSource := stream.GetSource()
	r.SDHash = hex.EncodeToString(streamSource.GetSdHash())
	defer os.RemoveAll(path.Join(c.blobsPath, r.SDHash))
	err = uploader.Upload(src)
	if err != nil {
		r.Retry = true
		return r, err
	}

	u, err := models.Users(
		models.UserWhere.ID.EQ(p.UserID),
		qm.Load(models.UserRels.LbrynetServer),
	).OneG()
	if err != nil {
		return r, fmt.Errorf("error getting sdk address for user %v: %w", p.UserID, err)
	}

	caller := query.NewCaller(u.R.LbrynetServer.Address, p.UserID)

	fileName := streamSource.Name
	if path.Ext(streamSource.Name) == "" {
		fileName += info.MediaType.Extension
	}
	// else {
	// 	fileName = strings.TrimSuffix(ss.Name, path.Ext(ss.Name)) + info.MediaType.Extension
	// }

	patch := map[string]interface{}{
		"file_size":            streamSource.Size,
		"file_name":            fileName,
		"file_hash":            hex.EncodeToString(streamSource.GetHash()),
		"sd_hash":              r.SDHash,
		"allow_duplicate_name": true,
	}
	m := info.MediaInfo
	if m != nil {
		patch["width"] = m.Width
		patch["height"] = m.Height
		patch["duration"] = m.Duration
	}
	pp := p.Request.Params.(map[string]interface{})
	for k, v := range patch {
		pp[k] = v
	}
	delete(pp, "file_path")
	p.Request.Params = pp
	res, err := caller.Call(context.Background(), p.Request)
	if err != nil {
		r.Retry = true
		return r, fmt.Errorf("error calling sdk: %w", err)
	}

	r.Response = res
	if res.Error != nil {
		return r, fmt.Errorf("sdk returned an error: %s", res.Error.Message)
	}
	return r, nil
}
