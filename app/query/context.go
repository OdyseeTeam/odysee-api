package query

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc/v2"
)

type contextKey string

var (
	contextKeyQuery    = contextKey("query")
	contextKeyResponse = contextKey("response")
	contextKeyLogEntry = contextKey("log-entry")
	contextKeyOrigin   = contextKey("origin")
)

func AttachQuery(ctx context.Context, query *Query) context.Context {
	return context.WithValue(ctx, contextKeyQuery, query)
}

func QueryFromContext(ctx context.Context) *Query {
	return ctx.Value(contextKeyQuery).(*Query)
}

func AttachResponse(ctx context.Context, response *jsonrpc.RPCResponse) context.Context {
	return context.WithValue(ctx, contextKeyResponse, response)
}

func ResponseFromContext(ctx context.Context) *jsonrpc.RPCResponse {
	return ctx.Value(contextKeyResponse).(*jsonrpc.RPCResponse)
}

func AttachLogEntry(ctx context.Context, entry *logrus.Entry) context.Context {
	return context.WithValue(ctx, contextKeyLogEntry, entry)
}

// WithLogField injects additional data into default post-query log entry
func WithLogField(ctx context.Context, key string, value interface{}) {
	e := ctx.Value(contextKeyLogEntry).(*logrus.Entry)
	e.Data[key] = value
}

func AttachOrigin(ctx context.Context, origin string) context.Context {
	return context.WithValue(ctx, contextKeyOrigin, origin)
}

func OriginFromContext(ctx context.Context) string {
	if origin, ok := ctx.Value(contextKeyOrigin).(string); ok {
		return origin
	}
	return ""
}
