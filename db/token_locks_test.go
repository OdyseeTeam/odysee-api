package db

import (
	"sync"
	"testing"
	"time"
)

var dummyTokens = map[string]int{}

func TestLockToken(t *testing.T) {
	var wg sync.WaitGroup

	for range [100]int{} {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			lockToken("token")
			time.Sleep(200 * time.Millisecond)
			releaseToken("token")
			wg.Done()
		}(&wg)
	}
	wg.Wait()
}
