package service

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

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
