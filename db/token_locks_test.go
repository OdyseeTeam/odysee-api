package db

import (
	"sync"
	"testing"
)

var dummyTokens = map[string]int{}

func TestWithValidAuthTokenConcurrent(t *testing.T) {
	var wg sync.WaitGroup

	for range [10]int{} {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			activeTokens.Lock("a")
			wg.Done()
		}(&wg)
	}
	wg.Wait()
}
