package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type serviceStatusSourceReporterStore struct {
	writes  chan time.Time
	release chan struct{}
}

func (s *serviceStatusSourceReporterStore) RecordSourceError(ctx context.Context, at time.Time) error {
	select {
	case s.writes <- at:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestServiceStatusSourceReporterDoesNotBlockOrLoseNewerGap(t *testing.T) {
	store := &serviceStatusSourceReporterStore{writes: make(chan time.Time, 2), release: make(chan struct{})}
	reporter := newServiceStatusSourceReporter(store)
	defer func() { require.NoError(t, reporter.stop()) }()
	first := time.Now().UTC()
	reporter.report(first)
	select {
	case at := <-store.writes:
		require.Equal(t, first, at)
	case <-time.After(3 * time.Second):
		t.Fatal("首个源缺口没有交给持久化 owner")
	}
	// 数据库正在处理旧时点时，新错误只能合并待写值，不能堵住请求。
	second := first.Add(time.Second)
	returned := make(chan struct{})
	go func() { reporter.report(second); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("源缺口报告阻塞了调用方")
	}
	close(store.release)
	select {
	case at := <-store.writes:
		require.Equal(t, second, at, "旧写入完成不能清空更新的待写缺口")
	case <-time.After(3 * time.Second):
		t.Fatal("更新的源缺口丢失")
	}
}

func TestServiceStatusSourceReporterStopFlushesLatestPending(t *testing.T) {
	store := &serviceStatusSourceReporterStore{writes: make(chan time.Time, 2), release: make(chan struct{})}
	reporter := newServiceStatusSourceReporter(store)
	defer func() { require.NoError(t, reporter.stop()) }()
	first := time.Now().UTC()
	reporter.report(first)
	require.Equal(t, first, <-store.writes)
	newer := first.Add(time.Second)
	reporter.report(newer)
	stopped := make(chan error, 1)
	go func() { stopped <- reporter.stop() }()
	select {
	case at := <-store.writes:
		require.Equal(t, newer, at, "停止必须用独立上下文提交最新源缺口")
		close(store.release)
	case <-stopped:
		t.Fatal("停止丢弃了尚未持久化的最新源缺口")
	case <-time.After(3 * time.Second):
		t.Fatal("停止未尝试持久化最新源缺口")
	}
	require.NoError(t, <-stopped)
}

func TestServiceStatusSourceReporterStopReportsUnfinishedWrite(t *testing.T) {
	store := &serviceStatusSourceReporterStore{writes: make(chan time.Time, 2), release: make(chan struct{})}
	reporter := newServiceStatusSourceReporter(store)
	reporter.report(time.Now())
	<-store.writes
	start := time.Now()
	require.ErrorIs(t, reporter.stop(), ErrServiceStatusUnavailable, "停止写入失败不得返回成功")
	require.Less(t, time.Since(start), 3*time.Second, "停止写入必须有界")
}
