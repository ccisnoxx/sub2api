package service

import (
	"context"
	"errors"
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

func (r *statusAggregatorRepo) GetConfig(context.Context) (*ServiceStatusConfig, error) {
	return &ServiceStatusConfig{Enabled: true, Platforms: []string{"openai"}}, nil
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
	require.NoError(t, a.Stop())
	require.NoError(t, a.Stop())
	require.True(t, r.stopped.Load())
	a.Start()
	require.Empty(t, r.called)
}

type statusAggregatorSwitchRepo struct {
	ServiceStatusRepository
	cfg       *ServiceStatusConfig
	err       error
	called    int
	remaining time.Duration
}

func (r *statusAggregatorSwitchRepo) GetConfig(context.Context) (*ServiceStatusConfig, error) {
	return r.cfg, r.err
}

func (r *statusAggregatorSwitchRepo) Aggregate(ctx context.Context, _ time.Time) error {
	r.called++
	deadline, _ := ctx.Deadline()
	r.remaining = time.Until(deadline)
	return nil
}

func TestServiceStatusAggregatorIndependentSwitchAndReenable(t *testing.T) {
	r := &statusAggregatorSwitchRepo{cfg: &ServiceStatusConfig{Platforms: []string{"openai"}}}
	a := NewServiceStatusAggregator(r)
	for _, enabled := range []bool{false, true, false, true} {
		r.cfg.Enabled = enabled
		before := r.called
		require.NoError(t, a.runOnce(context.Background(), time.Now().UTC()))
		if enabled {
			require.Equal(t, before+1, r.called)
			require.Greater(t, r.remaining, 44*time.Second)
			require.LessOrEqual(t, r.remaining, 45*time.Second)
		} else {
			require.Equal(t, before, r.called, "禁用轮次不执行聚合或保留清理")
		}
	}
	r.cfg.Platforms = []string{}
	require.NoError(t, a.runOnce(context.Background(), time.Now().UTC()))
	require.Equal(t, 2, r.called)
	r.err = errors.New("配置读取失败")
	require.ErrorIs(t, a.runOnce(context.Background(), time.Now().UTC()), r.err)
	require.Equal(t, 2, r.called, "读取失败不能猜测启用")
	r.err, r.cfg = nil, nil
	require.ErrorIs(t, a.runOnce(context.Background(), time.Now().UTC()), ErrServiceStatusUnavailable)
	require.Equal(t, 2, r.called)
}
