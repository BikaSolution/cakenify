package cronjob

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type tickerClient struct {
	interval time.Duration
	task     func()

	quit chan struct{}
	wg   sync.WaitGroup
}

// NewTicker creates a ticker-based cron job client that runs task every
// interval until Stop() is called or ctx is cancelled.
func NewTicker(interval time.Duration, task func()) Client {
	if interval <= 0 {
		panic(ErrIntervalMustBePositive)
	}
	if task == nil {
		panic(ErrTaskFunctionCannotBeNil)
	}

	return &tickerClient{
		interval: interval,
		task:     task,
		quit:     make(chan struct{}),
	}
}

func (tc *tickerClient) Start(ctx context.Context) {
	tc.wg.Add(1)
	go func() {
		defer tc.wg.Done()

		ticker := time.NewTicker(tc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				tc.executeTask()
			case <-tc.quit:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (tc *tickerClient) executeTask() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("cronjob task panicked", "recovered", r)
		}
	}()

	tc.task()
}

func (tc *tickerClient) Stop() {
	select {
	case <-tc.quit:
		return
	default:
		close(tc.quit)
	}
	tc.wg.Wait()
}
