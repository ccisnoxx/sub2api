package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

const statusCapacityInterval = time.Hour

// 计数属于当前进程，不替代外部日志保留与跨实例告警聚合。
type statusMonitor struct {
	mu                  sync.Mutex
	failures            int64
	consecutiveFailures int64
	lastProbe           time.Time
	lastCapacity        *statusCapacity
}

type statusAggregateStats struct {
	started         time.Time
	stage, outcome  string
	sourceRows      int64
	scanCompleted   bool
	factsWritten    int64
	cleanupStarted  bool
	cleanupDuration time.Duration
	deleted         [3]int64 // 事实、已解决事故、过期范围状态。
}

// 前置读取失败不进入业务事务，也不触发容量采样；分类与计数由同一 monitor 维护。
func (r *serviceStatusRepository) ReportAggregatePreflightFailure(ctx context.Context, started time.Time, err error) {
	r.monitor.reportAggregate(statusAggregateStats{started: started, stage: "config_preflight", outcome: "failed"}, err, ctx.Err())
}

func (m *statusMonitor) reportAggregate(s statusAggregateStats, err, ctxErr error) {
	duration := time.Since(s.started)
	code, alert := "", ""
	if err != nil {
		s.outcome, code = "failed", "aggregation_failed"
		switch {
		case errors.Is(ctxErr, context.Canceled):
			s.outcome, code = "cancelled", "cancelled"
		case errors.Is(ctxErr, context.DeadlineExceeded):
			code, alert = "aggregation_timeout", "aggregation_timeout"
		case errors.Is(err, service.ErrServiceStatusObservationLimit):
			code = "observation_limit"
		}
	}
	m.mu.Lock()
	if s.outcome == "failed" {
		m.failures++
		m.consecutiveFailures++
	} else if err == nil && s.outcome != "lock_skipped" {
		m.consecutiveFailures = 0
	}
	failures, consecutive := m.failures, m.consecutiveFailures
	m.mu.Unlock()
	if alert == "" && consecutive >= 3 && s.outcome == "failed" {
		alert = "aggregation_consecutive_failures"
	}
	if alert == "" && err == nil && duration >= 40*time.Second {
		alert = "aggregation_slow"
	}
	committed := err == nil && s.outcome != "lock_skipped"
	facts := int64(0)
	if committed {
		facts = s.factsWritten
	}
	fields := []zap.Field{
		zap.Bool(logger.OpsSystemLogSkipField, true),
		zap.String("event", "service_status_aggregate"), zap.String("outcome", s.outcome),
		zap.String("stage", s.stage), zap.String("code", code), zap.String("alert_code", alert),
		zap.Int64("duration_ms", duration.Milliseconds()), zap.Int64("source_rows_loaded", s.sourceRows),
		zap.Bool("scan_completed", s.scanCompleted), zap.Int64("facts_rows_committed", facts),
		zap.Int64("failures_total", failures), zap.Int64("consecutive_failures", consecutive),
	}
	l := logger.L().Named("service.service_status")
	if s.outcome == "failed" {
		l.Error("服务状态聚合失败", fields...)
	} else if alert != "" {
		l.Warn("服务状态聚合超过耗时预警线", fields...)
	} else {
		l.Info("服务状态聚合运行记录", fields...)
	}
	if s.cleanupStarted {
		deleted := [3]int64{}
		outcome := "rolled_back"
		if committed {
			deleted, outcome = s.deleted, "committed"
		} else if s.stage == "commit" {
			// COMMIT 返回错误时，连接中断可能使服务端最终结果无法确认。
			outcome = "commit_unconfirmed"
		}
		fields := []zap.Field{
			zap.Bool(logger.OpsSystemLogSkipField, true), zap.String("event", "service_status_cleanup"),
			zap.String("outcome", outcome), zap.String("failed_stage", s.stage),
			zap.Bool("commit_confirmed", committed),
			zap.Int64("duration_ms", s.cleanupDuration.Milliseconds()),
			zap.Int64("facts_deleted", deleted[0]), zap.Int64("incidents_deleted", deleted[1]),
			zap.Int64("scope_states_deleted", deleted[2]), zap.Int64("facts_delete_attempted", s.deleted[0]),
		}
		if committed {
			l.Info("服务状态保留清理已提交", fields...)
		} else {
			l.Error("服务状态保留清理未提交", fields...)
		}
	}
}

type statusCapacity struct {
	TodayRows, YesterdayRows               int64
	TableBytes, IndexBytes, TotalBytes     int64
	LiveTuplesEstimate, DeadTuplesEstimate int64
	LastAutovacuum                         *time.Time
	AutovacuumCount                        int64
	StatsReset                             *time.Time
	LongTransactions                       int64
	ActivityComplete                       bool
}

// 支持绑定到单个连接，在权限测试中观察当前角色，而不污染数据库连接池。
type statusCapacityDB interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func readStatusCapacity(ctx context.Context, db statusCapacityDB, now time.Time) (*statusCapacity, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	day := now.UTC().Truncate(24 * time.Hour)
	c := &statusCapacity{}
	// 主键首列支持两天范围扫描；逻辑事实计数不采用受十分钟重算影响的 n_tup_ins。
	err = tx.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE bucket_start >= $1), count(*) FILTER (WHERE bucket_start < $1)
 FROM service_status_facts_1m WHERE bucket_start >= $2 AND bucket_start < $3`, day, day.Add(-24*time.Hour), day.Add(24*time.Hour)).Scan(&c.TodayRows, &c.YesterdayRows)
	if err != nil {
		return nil, err
	}
	err = tx.QueryRowContext(ctx, `SELECT pg_table_size(relid),pg_indexes_size(relid),pg_total_relation_size(relid),n_live_tup,n_dead_tup,last_autovacuum,autovacuum_count,
 (SELECT stats_reset FROM pg_stat_database WHERE datname=current_database())
 FROM pg_stat_user_tables WHERE relid='service_status_facts_1m'::regclass`).Scan(&c.TableBytes, &c.IndexBytes, &c.TotalBytes, &c.LiveTuplesEstimate, &c.DeadTuplesEstimate, &c.LastAutovacuum, &c.AutovacuumCount, &c.StatsReset)
	if err != nil {
		return nil, err
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE xact_start < clock_timestamp()-interval '60 seconds'),
 (SELECT rolsuper OR pg_has_role(current_user,'pg_read_all_stats','USAGE') FROM pg_roles WHERE rolname=current_user)
 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid()`).Scan(&c.LongTransactions, &c.ActivityComplete)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return c, nil
}

// 大小增加超过 64 MiB 且超过前次 25% 时预警；这是初始提示线，不是容量 SLO。
func statusCapacityGrowth(previous, current *statusCapacity) bool {
	return previous != nil && current.TotalBytes-previous.TotalBytes > 64*1024*1024 &&
		float64(current.TotalBytes-previous.TotalBytes) > float64(previous.TotalBytes)*0.25
}

func (r *serviceStatusRepository) sampleStatusCapacity(parent context.Context) {
	now := time.Now()
	r.monitor.mu.Lock()
	if now.Sub(r.monitor.lastProbe) < statusCapacityInterval {
		r.monitor.mu.Unlock()
		return
	}
	r.monitor.lastProbe = now
	r.monitor.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	c, err := readStatusCapacity(ctx, r.db, now)
	l := logger.L().Named("service.service_status")
	if err != nil {
		// 不输出原始 SQL 错误、连接串或任何请求字段。
		l.Error("服务状态容量采样失败", zap.Bool(logger.OpsSystemLogSkipField, true),
			zap.String("event", "service_status_capacity"), zap.String("outcome", "failed"),
			zap.String("alert_code", "capacity_sample_failed"))
		return
	}
	r.monitor.mu.Lock()
	previous := r.monitor.lastCapacity
	r.monitor.lastCapacity = c
	r.monitor.mu.Unlock()
	alert := ""
	if statusCapacityGrowth(previous, c) {
		alert = "capacity_growth"
	}
	fields := []zap.Field{
		zap.Bool(logger.OpsSystemLogSkipField, true), zap.String("event", "service_status_capacity"),
		zap.String("outcome", "success"), zap.String("alert_code", alert),
		zap.Int64("duration_ms", time.Since(now).Milliseconds()), zap.Int("retention_days", 31),
		zap.Int64("today_rows", c.TodayRows), zap.Int64("yesterday_rows", c.YesterdayRows),
		zap.Int64("table_bytes", c.TableBytes), zap.Int64("index_bytes", c.IndexBytes), zap.Int64("total_bytes", c.TotalBytes),
		zap.Int64("live_tuples_estimate", c.LiveTuplesEstimate), zap.Int64("dead_tuples_estimate", c.DeadTuplesEstimate),
		zap.Any("last_autovacuum", c.LastAutovacuum), zap.Int64("autovacuum_count", c.AutovacuumCount),
		zap.Any("stats_reset", c.StatsReset), zap.Int64("long_transactions", c.LongTransactions),
		zap.Bool("activity_complete", c.ActivityComplete),
	}
	if alert != "" {
		l.Warn("服务状态事实表增长预警", fields...)
	} else {
		l.Info("服务状态容量与数据库维护采样", fields...)
	}
}
