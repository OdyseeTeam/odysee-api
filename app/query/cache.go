package query

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/pkg/chainquery"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/pierrec/lz4"
	"github.com/ybbus/jsonrpc/v2"
	"golang.org/x/sync/singleflight"
)

const (
	methodTagSeparator   = ":"
	invalidationInterval = 15 * time.Second
)

type CacheRequest struct {
	Method  string
	Params  any
	metaKey string
}

type CachedResponse struct {
	Result any
	Error  *jsonrpc.RPCError
}

type QueryCache struct {
	cache        *marshaler.Marshaler
	singleflight *singleflight.Group
	height       int
	stopChan     chan struct{}
}

func NewQueryCache(baseCache cache.CacheInterface[any]) *QueryCache {
	marshal := marshaler.New(baseCache)
	return &QueryCache{
		cache:        marshal,
		singleflight: &singleflight.Group{},
		stopChan:     make(chan struct{}),
	}
}

func NewQueryCacheWithInvalidator(baseCache cache.CacheInterface[any]) (*QueryCache, error) {
	qc := NewQueryCache(baseCache)
	height, err := chainquery.GetHeight()
	if err != nil {
		QueryCacheErrorCount.WithLabelValues(CacheAreaChainquery).Inc()
		return nil, fmt.Errorf("failed to get current height: %w", err)
	}
	qc.height = height
	go func() {
		ticker := time.NewTicker(invalidationInterval)
		for {
			select {
			case <-ticker.C:
				qc.runInvalidator()
			case <-qc.stopChan:
				return
			}
		}
	}()

	return qc, nil
}

func NewCacheRequest(method string, params any, metaKey string) CacheRequest {
	return CacheRequest{
		Method:  method,
		Params:  params,
		metaKey: metaKey,
	}
}

func (c *QueryCache) Retrieve(query *Query, metaKey string, getter func() (any, error)) (*CachedResponse, error) {
	log := logger.Log()

	cacheReq := NewCacheRequest(query.Method(), query.Params(), metaKey)

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

		// Cold object retrieval after cache miss
		log.Infof("cache miss for %s, key=%s, duration=%.2fs", cacheReq.Method, cacheReq.GetCacheKey(), time.Since(start).Seconds())
		start := time.Now()
		obj, err, _ := c.singleflight.Do(cacheReq.GetCacheKey(), getter)
		if err != nil {
			ObserveQueryCacheRetrievalDuration(CacheResultError, cacheReq.Method, start)
			return nil, fmt.Errorf("error calling getter: %w", err)
		}
		ObserveQueryCacheRetrievalDuration(CacheResultSuccess, cacheReq.Method, start)

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
				monitor.ErrorToSentry(fmt.Errorf("error during cache.set: %w", err), map[string]string{ParamMethod: cacheReq.Method})
				log.Warnf("error during cache.set (query returned): %s", err)
				return cacheResp, nil
			}
			ObserveQueryCacheOperation(CacheOperationSet, CacheResultSuccess, cacheReq.Method, start)
		}

		return cacheResp, nil
	}
	if hit == nil {
		ObserveQueryCacheOperation(CacheOperationGet, CacheResultError, cacheReq.Method, start)
		return nil, nil
	}
	log.Infof("cache hit for %s, key=%s, duration=%.2fs", cacheReq.Method, cacheReq.GetCacheKey(), time.Since(start).Seconds())
	ObserveQueryCacheOperation(CacheOperationGet, CacheResultHit, cacheReq.Method, start)
	cacheResp, ok := hit.(*CachedResponse)
	if !ok {
		return nil, errors.New("unknown cache object retrieved")
	}
	return cacheResp, nil
}

func (c *QueryCache) runInvalidator() error {
	log := logger.Log()
	height, err := chainquery.GetHeight()
	if err != nil {
		QueryCacheErrorCount.WithLabelValues(CacheAreaChainquery).Inc()
		return fmt.Errorf("failed to get current height: %w", err)
	}
	if c.height >= height {
		log.Infof("block height unchanged (%v = %v), cache invalidation skipped", height, c.height)
		return nil
	}

	log.Infof("new block height (%v > %v), running invalidation", height, c.height)
	c.height = height

	ctx, cancel := context.WithTimeout(context.Background(), invalidationInterval)
	defer cancel()
	err = c.cache.Invalidate(ctx, store.WithInvalidateTags(
		[]string{fmt.Sprintf("%s%s%s", ParamMethod, methodTagSeparator, MethodClaimSearch)},
	))
	if err != nil {
		QueryCacheErrorCount.WithLabelValues(CacheAreaInvalidateCall).Inc()
		log.Warnf("failed to invalidate %s entries: %s", MethodClaimSearch, err)
		return fmt.Errorf("failed to invalidate %s entries: %w", MethodClaimSearch, err)
	}

	return nil
}

func (r CacheRequest) Expiration() time.Duration {
	switch r.Method {
	case MethodResolve:
		return 600 * time.Second
	case MethodClaimSearch:
		return 160 * time.Second
	default:
		return 60 * time.Second
	}
}

func (r CacheRequest) Tags() []string {
	return []string{fmt.Sprintf("%s%s%s", ParamMethod, methodTagSeparator, r.Method)}
}

func (r CacheRequest) GetCacheKey() string {
	digester := crypto.MD5.New()
	var params string

	if r.Params == nil {
		params = "()"
	} else {
		if p, err := json.Marshal(r.Params); err != nil {
			params = "(error)"
		} else {
			params = string(p)
		}
	}
	fmt.Fprintf(digester, "[%s]%s:%s:%s", r.metaKey, "request", r.Method, params)
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
	val, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	vr := bytes.NewBuffer(val)
	vw := &bytes.Buffer{}
	zw := lz4.NewWriter(vw)
	_, err = io.Copy(zw, vr)
	zw.Close()
	if err != nil {
		return nil, err
	}
	return vw.Bytes(), nil
}

func (r *CachedResponse) UnmarshalBinary(data []byte) error {
	vr := bytes.NewBuffer(data)
	vw := &bytes.Buffer{}
	zr := lz4.NewReader(vr)
	_, err := io.Copy(vw, zr)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(vw)
	decoder.UseNumber()
	return decoder.Decode(r)
}
