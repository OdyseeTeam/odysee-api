package queue

import (
	"context"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/testdeps"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const busTestChannel = "test"

func TestQueueIntegration(t *testing.T) {
	redisHelper := testdeps.NewRedisTestHelper(t)

	queue, err := New(
		WithRequestsConnOpts(redisHelper.AsynqOpts),
		WithResponsesConnOpts(redisHelper.AsynqOpts),
		WithConcurrency(2),
		WithLogger(zapadapter.NewKV(nil)),
	)
	require.NoError(t, err)
	defer queue.Shutdown()

	results := make(chan struct{}, 1)

	queue.AddHandler(busTestChannel, func(ctx context.Context, task *asynq.Task) error {
		results <- struct{}{}
		return nil
	})

	go func() {
		err := queue.StartHandlers()
		assert.NoError(t, err)
	}()

	payload := map[string]interface{}{
		"key": "value",
	}
	err = queue.Put(busTestChannel, payload)
	assert.NoError(t, err)

	select {
	case <-results:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for task to be processed")
	}
}
