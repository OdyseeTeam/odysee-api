package bus

import (
	"context"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/testdeps"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
)

const busTestChannel = "test"

func TestBusIntegration(t *testing.T) {
	redisHelper := testdeps.NewRedisTestHelper(t)

	b, err := New(redisHelper.AsynqOpts, WithConcurrency(2))
	assert.NoError(t, err)
	defer b.Shutdown()

	results := make(chan struct{}, 1)

	b.AddHandler(busTestChannel, func(ctx context.Context, task *asynq.Task) error {
		results <- struct{}{}
		return nil
	})

	go func() {
		err := b.StartHandlers()
		assert.NoError(t, err)
	}()

	client, err := NewClient(redisHelper.AsynqOpts, zapadapter.NewKV(nil))
	assert.NoError(t, err)
	defer client.Close()

	payload := map[string]interface{}{
		"key": "value",
	}
	err = client.Put(busTestChannel, payload, 10, time.Second, time.Hour)
	assert.NoError(t, err)

	select {
	case <-results:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for task to be processed")
	}
}
