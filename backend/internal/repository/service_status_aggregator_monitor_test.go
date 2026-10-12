package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func statusMonitorLogs(c *statusLogCapture, name string) []*logger.LogEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	var events []*logger.LogEvent
	for _, event := range c.events {
		if event.Fields["event"] == name {
			events = append(events, event)
		}
	}
	return events
}

// 使用真实 Start/Stop 运行单轮；同一个仓储贯穿各轮，计数与正常 Aggregate 共用。
func runStatusAggregatorUntilReported(t *testing.T, r service.ServiceStatusRepository, c *statusLogCapture, want int, wait time.Duration) *logger.LogEvent {
	t.Helper()
	a := service.NewServiceStatusAggregator(r)
	t.Cleanup(func() { require.NoError(t, a.Stop()) })
	a.Start()
	require.Eventually(t, func() bool { return len(statusMonitorLogs(c, "service_status_aggregate")) >= want }, wait, time.Millisecond,
		"真实任务必须记录本轮聚合结果")
	require.NoError(t, a.Stop())
	events := statusMonitorLogs(c, "service_status_aggregate")
	require.Len(t, events, want, "每轮只报告一次")
	return events[want-1]
}

func statusPreflightConfigRows(enabled bool, platforms, epochs string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"version", "enabled", "platforms", "minimum_samples", "warning", "outage", "abnormal", "recovery", "epochs"}).
		AddRow(1, enabled, platforms, 5, .05, .9, 2, 3, epochs)
}

func TestServiceStatusAggregatorConfigFailureUsesRepositoryMonitor(t *testing.T) {
	c := captureStatusLogs(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	r := NewServiceStatusRepository(db)
	// 同一仓储的 HTTP 配置读取失败不属于任务轮次，不得污染监控计数。
	mock.ExpectQuery("SELECT version,enabled").WillReturnError(errors.New("private-http-config-detail"))
	_, err = service.NewServiceStatusService(r).GetConfig(context.Background())
	require.ErrorIs(t, err, service.ErrServiceStatusUnavailable)
	require.Empty(t, statusMonitorLogs(c, "service_status_aggregate"))
	mock.ExpectQuery("SELECT version,enabled").WillReturnError(errors.New("private-configuration-database-detail"))
	e := runStatusAggregatorUntilReported(t, r, c, 1, time.Second)
	require.Equal(t, "error", e.Level)
	require.Equal(t, "config_preflight", e.Fields["stage"])
	require.Equal(t, "failed", e.Fields["outcome"])
	require.Equal(t, "aggregation_failed", e.Fields["code"])
	require.Equal(t, int64(1), e.Fields["failures_total"])
	require.Equal(t, int64(1), e.Fields["consecutive_failures"])
	require.Equal(t, false, e.Fields["scan_completed"])
	require.Equal(t, int64(0), e.Fields["source_rows_loaded"])
	require.Equal(t, int64(0), e.Fields["facts_rows_committed"])
	require.Equal(t, true, e.Fields[logger.OpsSystemLogSkipField])
	require.NotContains(t, fmt.Sprint(e), "private-configuration-database-detail")
	require.Empty(t, statusMonitorLogs(c, "service_status_cleanup"))
	require.Empty(t, statusMonitorLogs(c, "service_status_capacity"))
	require.NoError(t, mock.ExpectationsWereMet())
}

type statusNilConfigRepository struct {
	service.ServiceStatusRepository
}

func (r *statusNilConfigRepository) GetConfig(context.Context) (*service.ServiceStatusConfig, error) {
	return nil, nil
}

func TestServiceStatusAggregatorPreflightFailuresShareCounterWithAggregateAndRecover(t *testing.T) {
	c := captureStatusLogs(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	r := NewServiceStatusRepository(db).(*serviceStatusRepository)
	// 容量采样的小时门控已独立验证；本用例只观察聚合失败与确认提交的共同计数。
	r.monitor.lastProbe = time.Now()
	for i := 1; i <= 3; i++ {
		mock.ExpectQuery("SELECT version,enabled").WillReturnError(errors.New("private-preflight-detail"))
		e := runStatusAggregatorUntilReported(t, r, c, i, time.Second)
		require.Equal(t, "error", e.Level)
		require.Equal(t, "config_preflight", e.Fields["stage"])
		require.Equal(t, int64(i), e.Fields["failures_total"])
		require.Equal(t, int64(i), e.Fields["consecutive_failures"])
		alert := ""
		if i == 3 {
			alert = "aggregation_consecutive_failures"
		}
		require.Equal(t, alert, e.Fields["alert_code"])
	}
	// nil 配置是任务的显式失败；报告仍由真实仓储的 monitor 实现。
	e := runStatusAggregatorUntilReported(t, &statusNilConfigRepository{r}, c, 4, time.Second)
	require.Equal(t, "error", e.Level)
	require.Equal(t, "config_preflight", e.Fields["stage"])
	require.Equal(t, "aggregation_failed", e.Fields["code"])
	require.Equal(t, int64(4), e.Fields["failures_total"])
	require.Equal(t, int64(4), e.Fields["consecutive_failures"])
	require.Empty(t, statusMonitorLogs(c, "service_status_cleanup"))
	require.Empty(t, statusMonitorLogs(c, "service_status_capacity"))

	epochs := fmt.Sprintf(`{"openai":%q}`, time.Now().UTC().Truncate(time.Minute).Add(time.Minute).Format(time.RFC3339))
	mock.ExpectQuery("SELECT version,enabled").WillReturnRows(statusPreflightConfigRows(true, `["openai"]`, epochs))
	mock.ExpectBegin().WillReturnError(errors.New("private-begin-detail"))
	mock.ExpectExec("UPDATE service_status_watermark SET last_error_code").WithArgs("SERVICE_STATUS_AGGREGATION_FAILED").WillReturnResult(sqlmock.NewResult(0, 1))
	e = runStatusAggregatorUntilReported(t, r, c, 5, time.Second)
	require.Equal(t, "begin", e.Fields["stage"])
	require.Equal(t, int64(5), e.Fields["failures_total"], "进入 Aggregate 后只由原报告路径计数")
	require.Equal(t, int64(5), e.Fields["consecutive_failures"])
	require.Equal(t, "aggregation_consecutive_failures", e.Fields["alert_code"])

	mock.ExpectQuery("SELECT version,enabled").WillReturnRows(statusPreflightConfigRows(true, `["openai"]`, epochs))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_try_advisory_xact_lock").WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(true))
	mock.ExpectQuery("SELECT version,enabled").WillReturnRows(statusPreflightConfigRows(true, `["openai"]`, epochs))
	for _, table := range []string{"service_status_facts_1m", "service_status_incidents", "service_status_scope_states"} {
		mock.ExpectExec("DELETE FROM " + table).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectQuery("SELECT observed_through").WillReturnRows(sqlmock.NewRows([]string{"observed_through"}).AddRow(nil))
	mock.ExpectCommit()
	e = runStatusAggregatorUntilReported(t, r, c, 6, time.Second)
	require.Equal(t, "awaiting_epoch", e.Fields["outcome"])
	require.Equal(t, "commit", e.Fields["stage"])
	require.Equal(t, "info", e.Level)
	require.Equal(t, int64(5), e.Fields["failures_total"])
	require.Equal(t, int64(0), e.Fields["consecutive_failures"], "确认提交后重置前置与事务失败的共同连续计数")
	require.Equal(t, "", e.Fields["alert_code"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServiceStatusAggregatorPreflightStopCancellationKeepsFailureCounter(t *testing.T) {
	c := captureStatusLogs(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	r := NewServiceStatusRepository(db)
	mock.ExpectQuery("SELECT version,enabled").WillReturnError(errors.New("private-preflight-detail"))
	runStatusAggregatorUntilReported(t, r, c, 1, time.Second)
	mock.ExpectQuery("SELECT version,enabled").WillDelayFor(time.Hour).WillReturnRows(statusPreflightConfigRows(false, `[]`, `{}`))
	a := service.NewServiceStatusAggregator(r)
	t.Cleanup(func() { require.NoError(t, a.Stop()) })
	a.Start()
	require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, time.Second, time.Millisecond,
		"先确认配置查询阻塞，再取消真实任务")
	require.NoError(t, a.Stop())
	events := statusMonitorLogs(c, "service_status_aggregate")
	require.Len(t, events, 2)
	e := events[1]
	require.Equal(t, "info", e.Level)
	require.Equal(t, "config_preflight", e.Fields["stage"])
	require.Equal(t, "cancelled", e.Fields["outcome"])
	require.Equal(t, "cancelled", e.Fields["code"])
	require.Equal(t, "", e.Fields["alert_code"])
	require.Equal(t, int64(1), e.Fields["failures_total"])
	require.Equal(t, int64(1), e.Fields["consecutive_failures"])
	a.Start()
	require.NoError(t, a.Stop())
	require.Len(t, statusMonitorLogs(c, "service_status_aggregate"), 2)
	require.Empty(t, statusMonitorLogs(c, "service_status_cleanup"))
	require.Empty(t, statusMonitorLogs(c, "service_status_capacity"))
}

func TestServiceStatusAggregatorPreflightDeadlineUsesSharedBudget(t *testing.T) {
	c := captureStatusLogs(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	r := NewServiceStatusRepository(db)
	for i := 1; i <= 2; i++ {
		mock.ExpectQuery("SELECT version,enabled").WillReturnError(errors.New("private-preflight-detail"))
		runStatusAggregatorUntilReported(t, r, c, i, time.Second)
	}
	mock.ExpectQuery("SELECT version,enabled").WillDelayFor(time.Minute).WillReturnRows(statusPreflightConfigRows(false, `[]`, `{}`))
	e := runStatusAggregatorUntilReported(t, r, c, 3, 50*time.Second)
	require.Equal(t, "error", e.Level)
	require.Equal(t, "config_preflight", e.Fields["stage"])
	require.Equal(t, "failed", e.Fields["outcome"])
	require.Equal(t, "aggregation_timeout", e.Fields["code"])
	require.Equal(t, "aggregation_timeout", e.Fields["alert_code"])
	require.GreaterOrEqual(t, e.Fields["duration_ms"], int64(45000))
	require.Equal(t, int64(3), e.Fields["failures_total"])
	require.Equal(t, int64(3), e.Fields["consecutive_failures"])
	require.Empty(t, statusMonitorLogs(c, "service_status_cleanup"))
	require.Empty(t, statusMonitorLogs(c, "service_status_capacity"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServiceStatusAggregatorRoundDurationIncludesConfigPreflight(t *testing.T) {
	c := captureStatusLogs(t)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	r := NewServiceStatusRepository(db).(*serviceStatusRepository)
	r.monitor.lastProbe = time.Now()
	epochs := fmt.Sprintf(`{"openai":%q}`, time.Now().UTC().Truncate(time.Minute).Add(time.Minute).Format(time.RFC3339))
	// 前置读取成功但耗时 41 秒；后续使用真实仓储的快速确认提交路径。
	mock.ExpectQuery("SELECT version,enabled").WillDelayFor(41 * time.Second).WillReturnRows(statusPreflightConfigRows(true, `["openai"]`, epochs))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_try_advisory_xact_lock").WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(true))
	mock.ExpectQuery("SELECT version,enabled").WillReturnRows(statusPreflightConfigRows(true, `["openai"]`, epochs))
	for _, table := range []string{"service_status_facts_1m", "service_status_incidents", "service_status_scope_states"} {
		mock.ExpectExec("DELETE FROM " + table).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectQuery("SELECT observed_through").WillReturnRows(sqlmock.NewRows([]string{"observed_through"}).AddRow(nil))
	mock.ExpectCommit()
	e := runStatusAggregatorUntilReported(t, r, c, 1, 44*time.Second)
	require.GreaterOrEqual(t, e.Fields["duration_ms"], int64(41000), "整轮耗时必须包含成功的前置配置读取")
	require.Equal(t, "aggregation_slow", e.Fields["alert_code"])
	require.Equal(t, "warn", e.Level)
	require.Equal(t, "awaiting_epoch", e.Fields["outcome"])
	require.Equal(t, "commit", e.Fields["stage"])
	require.Equal(t, "", e.Fields["code"])
	require.Equal(t, true, e.Fields[logger.OpsSystemLogSkipField])
	require.Equal(t, int64(0), e.Fields["failures_total"])
	require.Equal(t, int64(0), e.Fields["consecutive_failures"])
	keys := make([]string, 0, len(e.Fields))
	for key := range e.Fields {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, []string{"service", "env", logger.OpsSystemLogSkipField, "event", "outcome", "stage", "code", "alert_code", "duration_ms", "source_rows_loaded", "scan_completed", "facts_rows_committed", "failures_total", "consecutive_failures"}, keys,
		"运行日志仅包含固定白名单，不复制配置或请求字段")
	require.NoError(t, mock.ExpectationsWereMet())
}
