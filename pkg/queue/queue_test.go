package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/testdeps"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	queueRequest  = "test:request"
	queueResponse = "test:response"
)

func TestQueueIntegration(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	redisRequestsHelper := testdeps.NewRedisTestHelper(t, 1)
	redisResponsesHelper := testdeps.NewRedisTestHelper(t, 2)
	queue, err := New(
		WithRequestsConnOpts(redisRequestsHelper.AsynqOpts),
		WithResponsesConnOpts(redisResponsesHelper.AsynqOpts),
		WithConcurrency(2),
		WithLogger(zapadapter.NewKV(nil)),
	)
	require.NoError(err)
	defer queue.Shutdown()

	queueResponses, err := New(
		WithRequestsConnOpts(redisResponsesHelper.AsynqOpts),
		WithConcurrency(2),
		WithLogger(zapadapter.NewKV(nil)),
	)
	require.NoError(err)
	defer queueResponses.Shutdown()

	requests := make(chan map[string]any, 1)
	responses := make(chan map[string]any, 1)
	reqPayload := map[string]any{
		"request": "request",
	}
	respPayload := map[string]any{
		"response": "response",
	}

	queue.AddHandler(queueRequest, func(ctx context.Context, task *asynq.Task) error {
		assert.Equal(queueRequest, task.Type())
		var payload map[string]any
		err := json.Unmarshal(task.Payload(), &payload)
		assert.NoError(err)
		requests <- payload
		queue.SendResponse(queueResponse, payload)
		return nil
	})

	queueResponses.AddHandler(queueResponse, func(ctx context.Context, task *asynq.Task) error {
		assert.Equal(queueResponse, task.Type())
		var payload map[string]any
		err := json.Unmarshal(task.Payload(), &payload)
		assert.NoError(err)
		assert.Equal(reqPayload, payload)
		responses <- respPayload
		return nil
	})

	go func() {
		err := queue.ServeUntilShutdown()
		require.NoError(err)
	}()
	go func() {
		err := queueResponses.ServeUntilShutdown()
		require.NoError(err)
	}()

	err = queue.SendRequest(queueRequest, reqPayload)
	require.NoError(err)

	select {
	case rcPayload := <-requests:
		require.Equal(reqPayload, rcPayload)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for request to be processed")
	}

	select {
	case rcPayload := <-responses:
		require.Equal(respPayload, rcPayload)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for response to be sent")
	}
}
