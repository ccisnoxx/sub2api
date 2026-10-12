package service

import (
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

var serviceStatusSourceErrorAt atomic.Int64

type serviceStatusSourceErrorReporter struct{ report func(time.Time) }

var serviceStatusSourceReportCallback atomic.Pointer[serviceStatusSourceErrorReporter]

// SetServiceStatusSourceErrorReporter 注册唯一持久化 owner 的非阻塞回调。
// 回调只接收安全时点，不得同步访问数据库或等待队列；生命周期由聚合服务管理。
func SetServiceStatusSourceErrorReporter(report func(time.Time)) {
	if report == nil {
		serviceStatusSourceReportCallback.Store(nil)
		return
	}
	serviceStatusSourceReportCallback.Store(&serviceStatusSourceErrorReporter{report: report})
	if at := serviceStatusSourceErrorAt.Load(); at != 0 {
		report(time.Unix(0, at).UTC())
	}
}

// MarkServiceStatusSourceError 记录进程内已知源缺口，不记录请求键或原始错误。
// 聚合重启后仍受持久水位与启用区间约束；本信号不声称完整采样。
func MarkServiceStatusSourceError(at time.Time) {
	if at.IsZero() {
		at = time.Now()
	}
	value := at.UnixNano()
	for {
		previous := serviceStatusSourceErrorAt.Load()
		if value <= previous || serviceStatusSourceErrorAt.CompareAndSwap(previous, value) {
			break
		}
	}
	if reporter := serviceStatusSourceReportCallback.Load(); reporter != nil {
		reporter.report(time.Unix(0, serviceStatusSourceErrorAt.Load()).UTC())
	}
	reportServiceStatusSourceAlert("source")
}

func reportServiceStatusSourceAlert(stage string) {
	logger.L().Named("service.service_status").Error("服务状态源数据存在缺口",
		zap.Bool(logger.OpsSystemLogSkipField, true),
		zap.String("event", "service_status_source"), zap.String("stage", stage),
		zap.String("code", "SERVICE_STATUS_SOURCE_ERROR"), zap.String("alert_code", "source_error"))
}
func ServiceStatusSourceErrorSince(since time.Time) bool {
	at := serviceStatusSourceErrorAt.Load()
	return at != 0 && at >= since.UnixNano()
}
