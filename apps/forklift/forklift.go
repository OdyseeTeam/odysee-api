package forklift

import (
	"github.com/OdyseeTeam/odysee-api/app/asynquery"
	"github.com/OdyseeTeam/odysee-api/pkg/belt"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/hibiken/asynq"
)

// A list of task types.
const (
	TaskUpload       = "forklift:upload"
	TaskUploadResult = "forklift:result"
)

// type Forklift struct {
// 	b *belt.Belt
// }

func Start(blobsDstPath string, reflectorCfg map[string]string, redisOpts asynq.RedisConnOpt, logger logging.KVLogger) (*belt.Belt, error) {
	b, err := belt.New(redisOpts, belt.WithConcurrency(10))
	if err != nil {
		return nil, err
	}

	results := make(chan UploadResult)

	h, err := NewUploadHandler(blobsDstPath, results, reflectorCfg, logger)
	if err != nil {
		return nil, err
	}

	b.AddHandler(TaskUpload, h.HandleTask)

	go func() {
		for r := range results {
			if r.Error != "" {
				b.Put(TaskUploadResult, r, 10)
				continue
			}

			b.Put(asynquery.TaskAsyncQuery, asynquery.AsyncQuery{
				UserID:  r.UserID,
				Request: r.Request,
			}, 10)
		}
	}()

	return b, nil
}
