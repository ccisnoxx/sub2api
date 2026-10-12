package service

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type statusSourceLogSink struct{ events []*logger.LogEvent }

func (s *statusSourceLogSink) WriteLogEvent(e *logger.LogEvent) { s.events = append(s.events, e) }

func TestServiceStatusSourceAlertFixedFieldsAndPrivacy(t *testing.T) {
	logger.InitBootstrap()
	sink := &statusSourceLogSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })
	for _, stage := range []string{"source", "source_persist", "source_stop"} {
		reportServiceStatusSourceAlert(stage)
		e := sink.events[len(sink.events)-1]
		require.Equal(t, "error", e.Level)
		require.Equal(t, "service.service_status", e.LoggerName)
		require.Equal(t, stage, e.Fields["stage"])
		require.Equal(t, "source_error", e.Fields["alert_code"])
		require.Equal(t, "service_status_source", e.Fields["event"])
		require.Equal(t, "SERVICE_STATUS_SOURCE_ERROR", e.Fields["code"])
		require.Equal(t, true, e.Fields[logger.OpsSystemLogSkipField])
		// 完整白名单：不能新增请求、账号、范围标识或原始错误字段。
		for key := range e.Fields {
			require.Contains(t, []string{"service", "env", "event", "stage", "code", "alert_code", logger.OpsSystemLogSkipField}, key)
		}
	}
}

func TestServiceStatusSourceIntegrityFailureIsObservableAndReporterIsNonBlocking(t *testing.T) {
	previous := serviceStatusSourceErrorAt.Load()
	previousReporter := serviceStatusSourceReportCallback.Load()
	serviceStatusSourceErrorAt.Store(0)
	defer func() {
		serviceStatusSourceErrorAt.Store(previous)
		serviceStatusSourceReportCallback.Store(previousReporter)
	}()
	entered := make(chan time.Time, 1)
	SetServiceStatusSourceErrorReporter(func(at time.Time) {
		select {
		case entered <- at:
		default:
		}
	})
	at := time.Now().UTC()
	done := make(chan struct{})
	go func() { MarkServiceStatusSourceError(at); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("源失败报告不能阻塞请求或计费")
	}
	select {
	case got := <-entered:
		require.Equal(t, at, got)
	case <-time.After(time.Second):
		t.Fatal("未获得安全源失败报告")
	}
	require.True(t, ServiceStatusSourceErrorSince(at))
	require.False(t, ServiceStatusSourceErrorSince(at.Add(time.Nanosecond)))
	MarkServiceStatusSourceError(at.Add(-time.Minute))
	require.True(t, ServiceStatusSourceErrorSince(at))
	SetServiceStatusSourceErrorReporter(nil)
}
