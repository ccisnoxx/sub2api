package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type statusLogCapture struct {
	mu     sync.Mutex
	events []*logger.LogEvent
}

func (c *statusLogCapture) WriteLogEvent(e *logger.LogEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func captureStatusLogs(t *testing.T) *statusLogCapture {
	t.Helper()
	logger.InitBootstrap()
	c := &statusLogCapture{}
	logger.SetSink(c)
	t.Cleanup(func() { logger.SetSink(nil) })
	return c
}

func (c *statusLogCapture) last(t *testing.T, event string) *logger.LogEvent {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.events) - 1; i >= 0; i-- {
		if c.events[i].Fields["event"] == event {
			return c.events[i]
		}
	}
	t.Fatalf("缺少运行事件 %s", event)
	return nil
}

func TestServiceStatusMonitorFailureAlertAndRecovery(t *testing.T) {
	c := captureStatusLogs(t)
	m := &statusMonitor{}
	s := statusAggregateStats{started: time.Now(), stage: "scan", cleanupStarted: true, deleted: [3]int64{12, 3, 4}}
	for i := 0; i < 3; i++ {
		m.reportAggregate(s, errors.New("sensitive-database-detail"), nil)
	}
	e := c.last(t, "service_status_aggregate")
	require.Equal(t, "error", e.Level)
	require.Equal(t, int64(3), e.Fields["failures_total"])
	require.Equal(t, "aggregation_consecutive_failures", e.Fields["alert_code"])
	require.NotContains(t, fmt.Sprint(e), "sensitive-database-detail")
	cleanup := c.last(t, "service_status_cleanup")
	require.Equal(t, "rolled_back", cleanup.Fields["outcome"])
	require.Equal(t, int64(0), cleanup.Fields["facts_deleted"])
	require.Equal(t, int64(12), cleanup.Fields["facts_delete_attempted"])
	m.reportAggregate(statusAggregateStats{started: time.Now(), outcome: "lock_skipped"}, nil, nil)
	require.Equal(t, int64(3), c.last(t, "service_status_aggregate").Fields["consecutive_failures"])
	s.outcome = "success"
	m.reportAggregate(s, nil, nil)
	require.Equal(t, int64(0), c.last(t, "service_status_aggregate").Fields["consecutive_failures"])
	require.Equal(t, int64(3), c.last(t, "service_status_aggregate").Fields["failures_total"])
	require.Equal(t, int64(12), c.last(t, "service_status_cleanup").Fields["facts_deleted"])
}

func TestServiceStatusMonitorTimeoutCancellationAndSlowRun(t *testing.T) {
	c := captureStatusLogs(t)
	m := &statusMonitor{}
	s := statusAggregateStats{started: time.Now(), stage: "cleanup"}
	m.reportAggregate(s, service.ErrServiceStatusUnavailable, context.DeadlineExceeded)
	require.Equal(t, "aggregation_timeout", c.last(t, "service_status_aggregate").Fields["alert_code"])
	m.reportAggregate(s, service.ErrServiceStatusUnavailable, context.Canceled)
	e := c.last(t, "service_status_aggregate")
	require.Equal(t, "cancelled", e.Fields["outcome"])
	require.Equal(t, int64(1), e.Fields["failures_total"])
	s.started, s.outcome = time.Now().Add(-41*time.Second), "success"
	m.reportAggregate(s, nil, nil)
	require.Equal(t, "aggregation_slow", c.last(t, "service_status_aggregate").Fields["alert_code"])
}

func TestServiceStatusMonitorCapacityFailureDoesNotUndoCleanup(t *testing.T) {
	c := captureStatusLogs(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	r := &serviceStatusRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_try_advisory_xact_lock").WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(true))
	mock.ExpectQuery("SELECT version,enabled").WillReturnRows(sqlmock.NewRows([]string{"version", "enabled", "platforms", "minimum_samples", "warning", "outage", "abnormal", "recovery", "epochs"}).AddRow(1, false, `["openai"]`, 5, .05, .9, 2, 3, `{}`))
	for _, table := range []string{"service_status_facts_1m", "service_status_incidents", "service_status_scope_states"} {
		mock.ExpectExec("DELETE FROM " + table).WillReturnResult(sqlmock.NewResult(0, 7))
	}
	mock.ExpectCommit()
	// 独立容量事务失败必须显式记录，已提交的清理仍然成功。
	mock.ExpectBegin().WillReturnError(errors.New("private-connection-detail"))
	require.NoError(t, r.Aggregate(context.Background(), time.Now()))
	require.Equal(t, "disabled", c.last(t, "service_status_aggregate").Fields["outcome"])
	require.Equal(t, int64(7), c.last(t, "service_status_cleanup").Fields["facts_deleted"])
	e := c.last(t, "service_status_capacity")
	require.Equal(t, "capacity_sample_failed", e.Fields["alert_code"])
	require.NotContains(t, fmt.Sprint(e), "private-connection-detail")
	// 一个小时内不重复昂贵采样，失败事件保留给外部采集器。
	r.sampleStatusCapacity(context.Background())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServiceStatusMonitorCapacityGrowthThreshold(t *testing.T) {
	base := &statusCapacity{TotalBytes: 128 * 1024 * 1024}
	require.False(t, statusCapacityGrowth(nil, base))
	require.False(t, statusCapacityGrowth(base, &statusCapacity{TotalBytes: 192 * 1024 * 1024}))
	require.True(t, statusCapacityGrowth(base, &statusCapacity{TotalBytes: 193 * 1024 * 1024}))
	require.False(t, statusCapacityGrowth(base, &statusCapacity{TotalBytes: 32 * 1024 * 1024}))
	require.False(t, statusCapacityGrowth(&statusCapacity{TotalBytes: 1024 * 1024 * 1024}, &statusCapacity{TotalBytes: 1100 * 1024 * 1024}))
}
