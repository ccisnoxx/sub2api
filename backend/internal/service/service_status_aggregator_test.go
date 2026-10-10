package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type statusAggregatorRepo struct {
	ServiceStatusRepository
	called  chan time.Duration
	stopped atomic.Bool
}

func (r *statusAggregatorRepo) Aggregate(ctx context.Context, _ time.Time) error {
	deadline, _ := ctx.Deadline()
	r.called <- time.Until(deadline)
	<-ctx.Done()
	r.stopped.Store(true)
	return ctx.Err()
}
func TestServiceStatusAggregatorCancellationAndBudget(t *testing.T) {
	r := &statusAggregatorRepo{called: make(chan time.Duration, 1)}
	a := NewServiceStatusAggregator(r)
	a.Start()
	a.Start()
	select {
	case remaining := <-r.called:
		require.Greater(t, remaining, 44*time.Second)
		require.LessOrEqual(t, remaining, 45*time.Second)
	case <-time.After(time.Second):
		t.Fatal("聚合未启动")
	}
	a.Stop()
	a.Stop()
	require.True(t, r.stopped.Load())
	a.Start()
	require.Empty(t, r.called)
}
