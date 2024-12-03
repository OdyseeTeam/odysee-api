package sturdycache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/stretchr/testify/suite"
)

type ReplicatedCacheTestSuite struct {
	suite.Suite

	master          *miniredis.Miniredis
	replicas        []*miniredis.Miniredis
	cache           cache.CacheInterface[any]
	replicatedCache *ReplicatedCache
	teardownFunc    teardownFunc
	ctx             context.Context
	cancel          context.CancelFunc
}

type TestStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (t TestStruct) MarshalBinary() ([]byte, error) {
	return json.Marshal(t)
}

func (t *TestStruct) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, t)
}

func (s *ReplicatedCacheTestSuite) SetupTest() {
	var err error
	s.replicatedCache, s.master, s.replicas, s.teardownFunc = CreateTestCache(s.T())
	s.Require().NoError(err)
}

func (s *ReplicatedCacheTestSuite) TearDownTest() {
	s.teardownFunc()
}

func (s *ReplicatedCacheTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Second)
}

func (s *ReplicatedCacheTestSuite) TearDownSuite() {
	s.cancel()
}

func (s *ReplicatedCacheTestSuite) TestNewReplicatedCache() {
	s.Require().NotNil(s.cache)
	s.Require().NotNil(s.replicatedCache.masterCache)
	s.Require().Len(s.replicatedCache.replicaCaches, len(s.replicas))
}

func (s *ReplicatedCacheTestSuite) TestSet() {
	err := s.cache.Set(s.ctx, "key1", "value1")
	s.Require().NoError(err)

	val, err := s.master.Get("key1")
	s.Require().NoError(err)
	s.Require().Contains(val, "value1")

	err = s.cache.Set(s.ctx, "key2", "value2", store.WithExpiration(time.Minute))
	s.Require().NoError(err)

	ttl := s.master.TTL("key2")
	s.Require().True(ttl > 0)
}

func (s *ReplicatedCacheTestSuite) TestGet() {
	testKey := "test-key"
	testValue := "test-value"

	err := s.cache.Set(s.ctx, testKey, testValue)
	s.Require().NoError(err)

	masterValue, err := s.master.Get(testKey)
	s.Require().NoError(err)

	for _, r := range s.replicas {
		r.Set(testKey, masterValue)
	}

	value, err := s.cache.Get(s.ctx, testKey)
	s.Require().NoError(err)
	s.Require().Equal(testValue, value)
}

func (s *ReplicatedCacheTestSuite) TestGetWithReplicaFailures() {
	testKey := "test-key"
	testValue := "test-value"

	err := s.cache.Set(s.ctx, testKey, testValue)
	s.Require().NoError(err)

	// Manually replicate to replicas
	masterValue, err := s.master.Get(testKey)
	s.Require().NoError(err)

	for _, r := range s.replicas {
		r.Set(testKey, masterValue)
	}

	// Test with all replicas down
	value, err := s.cache.Get(s.ctx, testKey)
	s.Require().NoError(err)
	s.Require().Equal(testValue, value, "Should get value from master when all replicas are down")
}

func (s *ReplicatedCacheTestSuite) TestClear() {
	for i := range 3 {
		key := fmt.Sprintf("key-%d", i)
		err := s.cache.Set(s.ctx, key, fmt.Sprintf("value-%d", i))
		s.Require().NoError(err)
	}

	s.Require().Greater(len(s.master.Keys()), 0)

	err := s.cache.Clear(s.ctx)
	s.Require().NoError(err)

	s.Require().Equal(0, len(s.master.Keys()))
}

func (s *ReplicatedCacheTestSuite) TestInvalidate() {
	for i := range 5 {
		key := fmt.Sprintf("key-%d", i)
		err := s.cache.Set(s.ctx, key, fmt.Sprintf("value-%d", i), store.WithTags([]string{fmt.Sprintf("tag-%d", i)}))
		s.Require().NoError(err)
	}

	s.Require().Greater(len(s.master.Keys()), 0)

	err := s.cache.Invalidate(s.ctx, store.WithInvalidateTags([]string{"tag-1", "tag-2"}))
	s.Require().NoError(err)

	_, err = s.cache.Get(s.ctx, "key-1")
	s.Require().True(errors.Is(err, &store.NotFound{}))
	_, err = s.cache.Get(s.ctx, "key-2")
	s.Require().True(errors.Is(err, &store.NotFound{}))
}
func (s *ReplicatedCacheTestSuite) TestGetNonExistentKey() {
	_, err := s.cache.Get(s.ctx, "non-existent-key")
	s.Require().True(errors.Is(err, &store.NotFound{}))
}

func (s *ReplicatedCacheTestSuite) TestSetStructValue() {

	testValue := TestStruct{
		ID:   1,
		Name: "test",
	}

	err := s.cache.Set(s.ctx, "struct-key", testValue)
	s.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wait := time.NewTicker(100 * time.Millisecond)
Wait:
	for {
		select {
		case <-ctx.Done():
			s.FailNow("failed to read value")
		case <-wait.C:
			value, err := s.cache.Get(s.ctx, "struct-key")
			if err != nil {
				continue
			}
			s.Require().NoError(err)
			s.Require().NotNil(value)
			break Wait
		}
	}

}

func TestReplicatedCacheTestSuite(t *testing.T) {
	suite.Run(t, new(ReplicatedCacheTestSuite))
}
