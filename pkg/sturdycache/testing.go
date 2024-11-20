package sturdycache

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

type teardownFunc func()

func CreateTestCache(t *testing.T) (*ReplicatedCache, *miniredis.Miniredis, []*miniredis.Miniredis, teardownFunc) {
	require := require.New(t)
	master := miniredis.RunT(t)

	replicas := make([]*miniredis.Miniredis, 3)
	for i := range 3 {
		replicas[i] = miniredis.RunT(t)
	}

	replicaAddrs := make([]string, len(replicas))
	for i, r := range replicas {
		replicaAddrs[i] = r.Addr()
	}

	cache, err := NewReplicatedCache(
		master.Addr(),
		replicaAddrs,
		"",
	)
	require.NoError(err)
	return cache, master, replicas, func() {
		master.Close()
		for _, r := range replicas {
			r.Close()
		}
	}
}
