package service

import (
	"context"
	"sync"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type ServiceStatusAggregator struct {
	repo             ServiceStatusRepository
	mu               sync.Mutex
	started, stopped bool
	cancel           context.CancelFunc
	done             chan struct{}
	sourceReporter   *serviceStatusSourceReporter
}

func NewServiceStatusAggregator(repo ServiceStatusRepository) *ServiceStatusAggregator {
	return &ServiceStatusAggregator{repo: repo}
}
func (a *ServiceStatusAggregator) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started || a.stopped || a.repo == nil {
		return
	}
	a.started = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel, a.done = cancel, make(chan struct{})
	go a.loop(ctx)
}
func (a *ServiceStatusAggregator) Stop() error {
	a.mu.Lock()
	a.stopped = true
	if a.cancel != nil {
		a.cancel()
	}
	done := a.done
	a.mu.Unlock()
	if done != nil {
		<-done
	}
	if a.sourceReporter != nil {
		SetServiceStatusSourceErrorReporter(nil)
		return a.sourceReporter.stop()
	}
	return nil
}
func (a *ServiceStatusAggregator) loop(ctx context.Context) {
	defer close(a.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		run, cancel := context.WithTimeout(ctx, 45*time.Second)
		err := a.repo.Aggregate(run, time.Now().UTC())
		cancel()
		if err != nil && ctx.Err() == nil {
			// 仅输出固定原因，不记录原始数据库错误或源请求内容。
			logger.LegacyPrintf("service.service_status", "服务状态聚合失败 stage=aggregate code=%s", apperrors.Reason(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
