package e2etest

import (
	"context"
	"errors"
	"testing"
	"time"
)

// var ErrWaitDone = errors.New("done waiting")
var ErrWaitContinue = errors.New("keep waiting")

func Wait(t *testing.T, description string, duration, interval time.Duration, run func() error) {
	t.Helper()
	// Request stream and wait until it's available.
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	wait := time.NewTicker(interval)
Wait:
	for {
		select {
		case <-ctx.Done():
			t.Logf("%s is taking too long", description)
			t.FailNow()
		case <-wait.C:
			err := run()
			if err != nil {
				if !errors.Is(err, ErrWaitContinue) {
					t.Logf("%s failed: %v", description, err)
					t.FailNow()
				}
			} else {
				break Wait
			}
		}
	}
}
