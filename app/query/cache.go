package query

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	marshal := marshaler.New(store)
	return &QueryCache{
		cache:        marshal,
		singleflight: &singleflight.Group{},
	}
}

func (c *QueryCache) Retrieve(query *Query, getter func() (any, error)) (*CachedResponse, error) {
	log := logger.Log()
	cacheReq := CacheRequest{
		Method: query.Method(),
		Params: query.Params(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	start := time.Now()

	hit, err := c.cache.Get(ctx, cacheReq, &CachedResponse{})
	if err != nil {
		if !errors.Is(err, &store.NotFound{}) {
			ObserveQueryCacheOperation(CacheOperationGet, CacheResultError, cacheReq.Method, start)
			return nil, nil
		}

		ObserveQueryCacheOperation(CacheOperationGet, CacheResultMiss, cacheReq.Method, start)

		if getter == nil {
			log.Warnf("nil getter provided for %s", cacheReq.Method)
			return nil, nil
		}

		log.Infof("cache miss for %s, key=%s, duration=%.2fs", cacheReq.Method, cacheReq.GetCacheKey(), time.Since(start).Seconds())
		// Cold object retrieval after cache miss
		start := time.Now()
		obj, err, _ := c.singleflight.Do(cacheReq.GetCacheKey(), getter)
		if err != nil {
			ObserveQueryRetrievalDuration(CacheResultError, cacheReq.Method, start)
			return nil, fmt.Errorf("error calling getter: %w", err)
		}
		ObserveQueryRetrievalDuration(CacheResultSuccess, cacheReq.Method, start)

		res, ok := obj.(*jsonrpc.RPCResponse)
		if !ok {
			return nil, errors.New("unknown type returned by getter")
		}

		cacheResp := &CachedResponse{Result: res.Result, Error: res.Error}
		if res.Error != nil {
			log.Debugf("rpc error received (%s), not caching", cacheReq.Method)
		} else {
			start := time.Now()
			err = c.cache.Set(
				ctx, cacheReq, cacheResp,
				store.WithExpiration(cacheReq.Expiration()),
				store.WithTags(cacheReq.Tags()),
			)
			if err != nil {
				ObserveQueryCacheOperation(CacheOperationSet, CacheResultError, cacheReq.Method, start)
				monitor.ErrorToSentry(fmt.Errorf("error during cache.set: %w", err), map[string]string{"method": cacheReq.Method})
				log.Warnf("error during cache.set: %s", err)
				return cacheResp, nil
			}
			ObserveQueryCacheOperation(CacheOperationSet, CacheResultSuccess, cacheReq.Method, start)
		}

		return cacheResp, nil
	}
	log.Infof("cache hit for %s, key=%s, duration=%.2fs", cacheReq.Method, cacheReq.GetCacheKey(), time.Since(start).Seconds())
	ObserveQueryCacheOperation(CacheOperationGet, CacheResultHit, cacheReq.Method, start)
	cacheResp, ok := hit.(*CachedResponse)
	if !ok {
		return nil, errors.New("unknown cache object retrieved")
	}
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
		return caller.SendQuery(ctx, query)
	})
	if err != nil {
		return nil, rpcerrors.NewSDKError(err)
	}
	return cachedResp.RPCResponse(query.Request.ID), nil
}
