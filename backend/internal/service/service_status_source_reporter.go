package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type serviceStatusSourceErrorStore interface {
	RecordSourceError(context.Context, time.Time) error
}

// serviceStatusSourceReporter 合并源缺口时点；报告失败不会阻断请求或计费队列。
type serviceStatusSourceReporter struct {
	store    serviceStatusSourceErrorStore
	mu       sync.Mutex
	pending  time.Time
	signal   chan struct{}
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
	stopErr  error
}

func newServiceStatusSourceReporter(store serviceStatusSourceErrorStore) *serviceStatusSourceReporter {
	ctx, cancel := context.WithCancel(context.Background())
	r := &serviceStatusSourceReporter{store: store, signal: make(chan struct{}, 1), cancel: cancel, done: make(chan struct{})}
	go r.loop(ctx)
	return r
}

func (r *serviceStatusSourceReporter) report(at time.Time) {
	r.mu.Lock()
	if at.After(r.pending) {
		r.pending = at.UTC()
	}
	r.mu.Unlock()
	select {
	case r.signal <- struct{}{}:
	default:
	}
}

func (r *serviceStatusSourceReporter) stop() error {
	r.stopOnce.Do(func() {
		r.cancel()
		<-r.done
		r.mu.Lock()
		at := r.pending
		r.mu.Unlock()
		if at.IsZero() {
			return
		}
		// 生产者已停止；最终写入不能继承被取消的聚合上下文。
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := r.store.RecordSourceError(ctx, at); err != nil {
			r.stopErr = ErrServiceStatusUnavailable
			logger.LegacyPrintf("service.service_status", "服务状态源缺口停止未完成 stage=source_integrity code=SERVICE_STATUS_SOURCE_ERROR")
			return
		}
		r.mu.Lock()
		r.pending = time.Time{}
		r.mu.Unlock()
	})
	return r.stopErr
}

func (r *serviceStatusSourceReporter) loop(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.signal:
		case <-ticker.C:
		}
		r.mu.Lock()
		at := r.pending
		r.mu.Unlock()
		if at.IsZero() {
			continue
		}
		writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := r.store.RecordSourceError(writeCtx, at)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				logger.LegacyPrintf("service.service_status", "服务状态源缺口报告失败 stage=source_integrity code=SERVICE_STATUS_SOURCE_ERROR")
			}
			continue
		}
		r.mu.Lock()
		if !r.pending.After(at) {
			r.pending = time.Time{}
		}
		r.mu.Unlock()
	}
}
