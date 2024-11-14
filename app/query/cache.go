package query

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/pkg/rpcerrors"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/ybbus/jsonrpc"
	"golang.org/x/sync/singleflight"
)

type CacheRequest struct {
	Method string
	Params any
}

type CachedResponse struct {
	Result any
	Error  *jsonrpc.RPCError
}

type QueryCache struct {
	cache        *marshaler.Marshaler
	singleflight *singleflight.Group
}

func NewQueryCache(store cache.CacheInterface[any]) *QueryCache {
	m := marshaler.New(store)
	return &QueryCache{
		cache:        m,
		singleflight: &singleflight.Group{},
	}
}

func (c *QueryCache) Retrieve(query *Query, getter func() (any, error)) (*CachedResponse, error) {
	log := logger.Log()
	cacheReq := CacheRequest{
		Method: query.Method(),
		Params: query.Params(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()

	hit, err := c.cache.Get(ctx, cacheReq, &CachedResponse{})
	if err != nil {
		if !errors.Is(err, &store.NotFound{}) {
			metrics.SturdyQueryCacheErrorCount.WithLabelValues(cacheReq.Method).Inc()
			return nil, fmt.Errorf("failed to cache.get: %w", err)
		}
		metrics.SturdyQueryCacheHitCount.WithLabelValues(cacheReq.Method).Inc()
		if getter == nil {
			log.Warnf("nil getter provided for %s", query.Method())
			metrics.SturdyQueryCacheErrorCount.WithLabelValues(cacheReq.Method).Inc()
			return nil, errors.New("cache miss with no object getter provided")
		}

		log.Infof("cold cache retrieval for %s", query.Method())
		obj, err, _ := c.singleflight.Do(cacheReq.GetCacheKey(), getter)
		if err != nil {
			metrics.SturdyQueryCacheErrorCount.WithLabelValues(cacheReq.Method).Inc()
			return nil, fmt.Errorf("failed to call object getter: %w", err)
		}
		res, ok := obj.(*jsonrpc.RPCResponse)
		if !ok {
			return nil, errors.New("unknown type returned by getter")
		}
		cacheResp := &CachedResponse{Result: res.Result, Error: res.Error}
		err = c.cache.Set(
			ctx, cacheReq, cacheResp,
			store.WithExpiration(cacheReq.Expiration()),
			store.WithTags(cacheReq.Tags()),
		)
		if err != nil {
			metrics.SturdyQueryCacheErrorCount.WithLabelValues(cacheReq.Method).Inc()
			monitor.ErrorToSentry(fmt.Errorf("failed to cache.set: %w", err), map[string]string{"method": cacheReq.Method})
			return nil, fmt.Errorf("failed to cache.set: %w", err)
		}
		return cacheResp, nil
	}
	cacheResp, ok := hit.(*CachedResponse)
	if !ok {
		metrics.SturdyQueryCacheErrorCount.WithLabelValues(cacheReq.Method).Inc()
		return nil, errors.New("unknown cache object retrieved")
	}
	metrics.SturdyQueryCacheHitCount.WithLabelValues(cacheReq.Method).Inc()
	return cacheResp, nil
}

func (r CacheRequest) Expiration() time.Duration {
	switch r.Method {
	case MethodResolve:
		return 600 * time.Second
	case MethodClaimSearch:
		return 180 * time.Second
	default:
		return 60 * time.Second
	}
}

func (r CacheRequest) Tags() []string {
	return []string{"method:" + r.Method}
}

func (r CacheRequest) GetCacheKey() string {
	digester := crypto.MD5.New()
	var params string

	if r.Params == nil {
		params = "()"
	} else {
		if p, err := json.Marshal(r.Params); err != nil {
			params = "(x)"
		} else {
			params = string(p)
		}
	}
	fmt.Fprintf(digester, "%s:%s:%s", "request", r.Method, params)
	hash := digester.Sum(nil)
	return fmt.Sprintf("%x", hash)
}

func (r *CachedResponse) RPCResponse(id int) *jsonrpc.RPCResponse {
	return &jsonrpc.RPCResponse{
		JSONRPC: "2.0",
		Result:  r.Result,
		Error:   r.Error,
		ID:      id,
	}
}

func (r *CachedResponse) MarshalBinary() ([]byte, error) {
	return json.Marshal(r)
}

func (r *CachedResponse) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, r)
}

func preflightCacheHook(caller *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
	log := logger.Log()
	if caller.Cache == nil {
		log.Warn("no cache present on caller")
		return nil, nil
	}
	query := QueryFromContext(ctx)
	cachedResp, err := caller.Cache.Retrieve(query, func() (any, error) {
		log.Debugf("cache miss, calling %s", query.Method())
		return caller.SendQuery(ctx, query)
	})
	if err != nil {
		return nil, rpcerrors.NewSDKError(err)
	}
	log.Debugf("cache hit for %s", query.Method())
	return cachedResp.RPCResponse(query.Request.ID), nil
}
