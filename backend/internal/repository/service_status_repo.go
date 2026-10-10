package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const statusLockID int64 = 7343400301
const statusBoundedNotice = "仅描述本部署已观察请求；日志缺失、被过滤或丢弃及超出十分钟重算/九十分钟关联边界的迟到记录无法重建。"

type serviceStatusRepository struct{ db *sql.DB }

func NewServiceStatusRepository(db *sql.DB) service.ServiceStatusRepository {
	return &serviceStatusRepository{db: db}
}
func (r *serviceStatusRepository) RecordSourceError(ctx context.Context, at time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE service_status_watermark SET source_error_at=GREATEST(source_error_at,$1) WHERE id=1`, at.UTC())
	if err != nil {
		return service.ErrServiceStatusUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return service.ErrServiceStatusUnavailable
	}
	return nil
}

type statusQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
type statusConfig struct {
	service.ServiceStatusConfig
	epochs map[string]time.Time
}

func loadStatusConfig(ctx context.Context, q statusQuerier, lock bool) (statusConfig, error) {
	var c statusConfig
	var platforms, epochs []byte
	query := `SELECT version,enabled,platforms,minimum_samples,warning_error_rate,outage_error_rate,abnormal_windows,recovery_windows,observation_starts FROM service_status_config WHERE id=1`
	if lock {
		query += ` FOR UPDATE`
	}
	err := q.QueryRowContext(ctx, query).Scan(&c.Version, &c.Enabled, &platforms, &c.MinimumSamples, &c.WarningErrorRate, &c.OutageErrorRate, &c.AbnormalWindows, &c.RecoveryWindows, &epochs)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(platforms, &c.Platforms); err != nil {
		return c, err
	}
	if err = json.Unmarshal(epochs, &c.epochs); err != nil {
		return c, err
	}
	if err = service.ValidateServiceStatusConfig(&c.ServiceStatusConfig); err != nil {
		return c, err
	}
	if c.Enabled {
		for _, p := range c.Platforms {
			if c.epochs[p].IsZero() {
				return c, service.ErrServiceStatusUnavailable
			}
		}
	}
	return c, nil
}
func (r *serviceStatusRepository) GetConfig(ctx context.Context) (*service.ServiceStatusConfig, error) {
	c, err := loadStatusConfig(ctx, r.db, false)
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	return &c.ServiceStatusConfig, nil
}
func (r *serviceStatusRepository) UpdateConfig(ctx context.Context, c *service.ServiceStatusConfig) (*service.ServiceStatusConfig, error) {
	if err := service.ValidateServiceStatusConfig(c); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	old, err := loadStatusConfig(ctx, tx, true)
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	if old.Version != c.Version {
		return nil, service.ErrServiceStatusConfigConflict
	}
	var now time.Time
	if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	epochs := map[string]time.Time{}
	if c.Enabled {
		for _, p := range c.Platforms {
			e, ok := old.epochs[p]
			if !old.Enabled || !ok {
				e = now.UTC().Truncate(time.Minute).Add(time.Minute)
			}
			epochs[p] = e
		}
	}
	pJSON, _ := json.Marshal(c.Platforms)
	eJSON, _ := json.Marshal(epochs)
	_, err = tx.ExecContext(ctx, `UPDATE service_status_config SET version=version+1,enabled=$1,platforms=$2,minimum_samples=$3,warning_error_rate=$4,outage_error_rate=$5,abnormal_windows=$6,recovery_windows=$7,observation_starts=$8 WHERE id=1`, c.Enabled, string(pJSON), c.MinimumSamples, c.WarningErrorRate, c.OutageErrorRate, c.AbnormalWindows, c.RecoveryWindows, string(eJSON))
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	// 配置变化不得把已有正常统计当作新恢复证据；基线使用配置事务实际时刻。
	_, err = tx.ExecContext(ctx, `UPDATE service_status_scope_states SET abnormal_count=0,recovery_count=0,last_evidence_at=GREATEST(last_evidence_at,$1),config_version=$2,last_health='unknown'`, now, c.Version+1)
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	if err = freezeStatusIncidents(ctx, tx, now); err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	if err = tx.Commit(); err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	result := *c
	result.Version++
	result.Platforms = append([]string{}, c.Platforms...)
	return &result, nil
}

func (r *serviceStatusRepository) Aggregate(ctx context.Context, now time.Time) (result error) {
	// 错误记录独立于已回滚事务，只保存固定码；失败从不刷新成功水位。
	defer func() {
		if result == nil {
			return
		}
		code := "SERVICE_STATUS_AGGREGATION_FAILED"
		if errors.Is(result, service.ErrServiceStatusObservationLimit) {
			code = "SERVICE_STATUS_OBSERVATION_LIMIT"
		} else if ctx.Err() != nil {
			code = "SERVICE_STATUS_AGGREGATION_TIMEOUT"
		}
		failureCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = r.db.ExecContext(failureCtx, `UPDATE service_status_watermark SET last_error_code=$1 WHERE id=1`, code)
	}()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return service.ErrServiceStatusUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	var acquired bool
	if err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock($1)`, statusLockID).Scan(&acquired); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	if !acquired {
		return nil
	}
	cfg, err := loadStatusConfig(ctx, tx, true)
	if err != nil {
		return service.ErrServiceStatusUnavailable
	}
	if err = pruneStatus(ctx, tx, now); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	if !cfg.Enabled || len(cfg.Platforms) == 0 {
		if err = tx.Commit(); err != nil {
			return service.ErrServiceStatusUnavailable
		}
		return nil
	}
	end := now.UTC().Truncate(time.Minute)
	start := end.Add(-10 * time.Minute)
	var previousThrough *time.Time
	if err = tx.QueryRowContext(ctx, `SELECT observed_through FROM service_status_watermark WHERE id=1`).Scan(&previousThrough); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	if previousThrough != nil && previousThrough.After(end) {
		return service.ErrServiceStatusUnavailable
	}
	for _, e := range cfg.epochs {
		if e.Before(end) {
			goto scan
		}
	}
	if err = tx.Commit(); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	return nil
scan:
	rows, err := loadStatusSources(ctx, tx, start.Add(-90*time.Minute), now)
	if err != nil {
		return service.ErrServiceStatusUnavailable
	}
	observations, err := normalizeStatusRows(rows, start, end, cfg.epochs)
	if err != nil {
		return err
	}
	sourceGap := ""
	var sourceErrorAt *time.Time
	if err = tx.QueryRowContext(ctx, `SELECT source_error_at FROM service_status_watermark WHERE id=1`).Scan(&sourceErrorAt); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	if service.ServiceStatusSourceErrorSince(start) || (sourceErrorAt != nil && !sourceErrorAt.Before(start)) {
		sourceGap = "source_error"
	}
	for p, epoch := range cfg.epochs {
		from := start
		if epoch.After(from) {
			from = epoch
		}
		if !from.Before(end) {
			continue
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM service_status_facts_1m WHERE platform=$1 AND bucket_start >= $2 AND bucket_start < $3`, p, from, end); err != nil {
			return service.ErrServiceStatusUnavailable
		}
	}
	minEpoch := end
	for _, e := range cfg.epochs {
		if e.Before(minEpoch) {
			minEpoch = e
		}
	}
	if minEpoch.Before(start) {
		minEpoch = start
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM service_status_facts_1m WHERE platform='' AND bucket_start >= $1 AND bucket_start < $2`, minEpoch, end); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	for _, f := range statusFacts(observations) {
		if sourceGap != "" {
			f.gap = sourceGap
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success,failure,excluded,unknown,last_qualified_at,last_success_at,gap_code) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, f.bucket, f.scope.platform, f.scope.groupID, f.scope.model, f.counts.Success, f.counts.Failure, f.counts.Excluded, f.counts.Unknown, f.lastQualified, f.lastSuccess, f.gap)
		if err != nil {
			return service.ErrServiceStatusUnavailable
		}
	}
	var oldCoverage []byte
	if err = tx.QueryRowContext(ctx, `SELECT coverage FROM service_status_watermark WHERE id=1`).Scan(&oldCoverage); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	var coverage []service.ServiceStatusCoverage
	if json.Unmarshal(oldCoverage, &coverage) != nil {
		return service.ErrServiceStatusUnavailable
	}
	coverage = updateStatusCoverage(coverage, cfg.epochs, start, end, now.Add(-31*24*time.Hour), sourceGap == "")
	encoded, _ := json.Marshal(coverage)
	if err = advanceStatusScopes(ctx, tx, cfg, end, sourceGap, coverage); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	_, err = tx.ExecContext(ctx, `UPDATE service_status_watermark SET observed_through=$1,config_version=$2,coverage=$3,gap_code=$4,last_error_code='' WHERE id=1`, end, cfg.Version, string(encoded), sourceGap)
	if err != nil {
		return service.ErrServiceStatusUnavailable
	}
	if err = tx.Commit(); err != nil {
		return service.ErrServiceStatusUnavailable
	}
	return nil
}

func pruneStatus(ctx context.Context, tx *sql.Tx, now time.Time) error {
	cutoff := now.Add(-31 * 24 * time.Hour)
	for _, query := range []string{
		`DELETE FROM service_status_facts_1m WHERE bucket_start < $1`,
		`DELETE FROM service_status_incidents WHERE resolved_at < $1`,
		`DELETE FROM service_status_scope_states s WHERE last_observation_at < $1 AND NOT EXISTS(SELECT 1 FROM service_status_incidents i WHERE i.platform=s.platform AND i.group_id=s.group_id AND i.requested_model=s.requested_model AND i.resolved_at IS NULL)`,
	} {
		if _, err := tx.ExecContext(ctx, query, cutoff); err != nil {
			return err
		}
	}
	return nil
}
func loadStatusSources(ctx context.Context, tx *sql.Tx, start, end time.Time) ([]statusSourceRow, error) {
	// 普通组平台由组定义，复合组必须有实际最终账号平台；没有账号不猜测。
	rows, err := tx.QueryContext(ctx, `
WITH candidate_keys AS (
 SELECT service_status_observation->>'observation_key' AS key FROM usage_logs
 WHERE created_at >= $1 AND created_at < $2 AND (created_at >= $3 OR (left(service_status_observation->>'observed_at',16) >= $4 AND left(service_status_observation->>'observed_at',16) < $5))
 UNION
 SELECT service_status_observation->>'observation_key' AS key FROM ops_error_logs
 WHERE created_at >= $1 AND created_at < $2 AND (created_at >= $3 OR (left(service_status_observation->>'observed_at',16) >= $4 AND left(service_status_observation->>'observed_at',16) < $5))
)
SELECT 'usage_logs',u.id,u.created_at,COALESCE(g.id,0),COALESCE(NULLIF(u.requested_model,''),u.model,''),
 CASE WHEN g.platform='composite' THEN COALESCE(a.platform,'') ELSE COALESCE(g.platform,'') END,
 COALESCE(g.status='active' AND g.deleted_at IS NULL,false),false,u.request_type,u.completion_status,u.is_complete,u.service_status_observation
 FROM usage_logs u LEFT JOIN groups g ON g.id=u.group_id LEFT JOIN accounts a ON a.id=u.account_id WHERE u.created_at >= $1 AND u.created_at < $2 AND (u.created_at >= $3 OR u.service_status_observation->>'observation_key' IN(SELECT key FROM candidate_keys))
UNION ALL
SELECT 'ops_error_logs',o.id,o.created_at,COALESCE(g.id,0),COALESCE(NULLIF(o.requested_model,''),o.model,''),
 CASE WHEN g.platform='composite' THEN COALESCE(a.platform,'') ELSE COALESCE(g.platform,'') END,
 COALESCE(g.status='active' AND g.deleted_at IS NULL,false),o.is_count_tokens,COALESCE(o.request_type,0),'',NULL,o.service_status_observation
 FROM ops_error_logs o LEFT JOIN groups g ON g.id=o.group_id LEFT JOIN accounts a ON a.id=o.account_id WHERE o.created_at >= $1 AND o.created_at < $2 AND (o.created_at >= $3 OR o.service_status_observation->>'observation_key' IN(SELECT key FROM candidate_keys))`, start, end, start.Add(90*time.Minute), start.Add(90*time.Minute).UTC().Format("2006-01-02T15:04"), end.UTC().Truncate(time.Minute).Format("2006-01-02T15:04"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []statusSourceRow
	for rows.Next() {
		var r statusSourceRow
		if err = rows.Scan(&r.table, &r.id, &r.created, &r.scope.groupID, &r.scope.model, &r.scope.platform, &r.visible, &r.countTokens, &r.requestType, &r.completion, &r.complete, &r.metadata); err != nil {
			return nil, err
		}
		if !service.ServiceStatusPlatformKnown(r.scope.platform) {
			r.scope.platform = ""
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func updateStatusCoverage(old []service.ServiceStatusCoverage, epochs map[string]time.Time, start, end, cutoff time.Time, covered bool) []service.ServiceStatusCoverage {
	var result []service.ServiceStatusCoverage
	for _, c := range old {
		if c.Start.Before(cutoff) {
			c.Start = cutoff
		}
		if !c.Start.Before(c.End) {
			continue
		}
		from, active := epochs[c.Platform]
		if from.Before(start) {
			from = start
		}
		if active && from.Before(end) && c.Start.Before(end) && c.End.After(from) {
			if c.Start.Before(from) {
				left := c
				left.End = from
				result = append(result, left)
			}
			if c.End.After(end) {
				right := c
				right.Start = end
				result = append(result, right)
			}
		} else {
			result = append(result, c)
		}
	}
	if covered {
		for p, epoch := range epochs {
			from := start
			if epoch.After(from) {
				from = epoch
			}
			if from.Before(end) {
				result = append(result, service.ServiceStatusCoverage{Platform: p, ServiceStatusInterval: service.ServiceStatusInterval{Start: from, End: end}})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Platform != result[j].Platform {
			return result[i].Platform < result[j].Platform
		}
		return result[i].Start.Before(result[j].Start)
	})
	merged := make([]service.ServiceStatusCoverage, 0, len(result))
	for _, c := range result {
		n := len(merged)
		if n > 0 && merged[n-1].Platform == c.Platform && !c.Start.After(merged[n-1].End) {
			if c.End.After(merged[n-1].End) {
				merged[n-1].End = c.End
			}
		} else {
			merged = append(merged, c)
		}
	}
	return merged
}
func statusCovered(coverage []service.ServiceStatusCoverage, p string, start, end time.Time) bool {
	for _, c := range coverage {
		if c.Platform == p && !c.Start.After(start) && !c.End.Before(end) {
			return true
		}
	}
	return false
}

func addStatusCounts(a *service.ServiceStatusCounts, b service.ServiceStatusCounts) {
	a.Success += b.Success
	a.Failure += b.Failure
	a.Excluded += b.Excluded
	a.Unknown += b.Unknown
}
func loadStatusFacts(ctx context.Context, q statusQuerier, platforms []string, start, end time.Time) ([]statusFact, error) {
	rows, err := q.QueryContext(ctx, `SELECT f.bucket_start,f.platform,f.group_id,f.requested_model,f.success,f.failure,f.excluded,f.unknown,f.last_qualified_at,f.last_success_at,f.gap_code FROM service_status_facts_1m f LEFT JOIN groups g ON g.id=f.group_id WHERE f.bucket_start >= $1 AND f.bucket_start < $2 AND (f.platform=ANY($3) OR f.platform='') AND (f.group_id=0 OR (g.status='active' AND g.deleted_at IS NULL))`, start, end, pq.Array(platforms))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []statusFact
	for rows.Next() {
		var f statusFact
		if err = rows.Scan(&f.bucket, &f.scope.platform, &f.scope.groupID, &f.scope.model, &f.counts.Success, &f.counts.Failure, &f.counts.Excluded, &f.counts.Unknown, &f.lastQualified, &f.lastSuccess, &f.gap); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

func (r *serviceStatusRepository) Snapshot(ctx context.Context, historyRange, platform string) (*service.ServiceStatusSnapshot, error) {
	if historyRange == "" {
		historyRange = "24h"
	}
	duration, step := 24*time.Hour, time.Hour
	switch historyRange {
	case "24h":
	case "7d":
		duration, step = 7*24*time.Hour, 6*time.Hour
	case "30d":
		duration, step = 30*24*time.Hour, 24*time.Hour
	default:
		return nil, service.ErrServiceStatusInvalidFilter
	}
	if platform != "" && !service.ServiceStatusPlatformKnown(platform) {
		return nil, service.ErrServiceStatusInvalidFilter
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	cfg, err := loadStatusConfig(ctx, tx, false)
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	if !cfg.Enabled {
		return nil, service.ErrServiceStatusDisabled
	}
	var now time.Time
	if err = tx.QueryRowContext(ctx, `SELECT transaction_timestamp()`).Scan(&now); err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	now = now.UTC()
	platforms := append([]string{}, cfg.Platforms...)
	if platform != "" {
		platforms = []string{}
		for _, p := range cfg.Platforms {
			if p == platform {
				platforms = append(platforms, p)
			}
		}
	}
	s := &service.ServiceStatusSnapshot{SchemaVersion: 1, ConfigVersion: cfg.Version, GeneratedAt: now, MonitoringEnabled: true, EnabledPlatforms: platforms, Coverage: []service.ServiceStatusCoverage{}, Gaps: []string{}, Platforms: []service.ServiceStatusPlatform{}, Scopes: []service.ServiceStatusLeaf{}, History: []service.ServiceStatusHistoryPoint{}, Incidents: []service.ServiceStatusIncident{}, BoundedObservationNotice: statusBoundedNotice}
	s.HistoryRange = service.ServiceStatusInterval{Start: now.Truncate(time.Minute).Add(-duration), End: now.Truncate(time.Minute)}
	var coverageRaw []byte
	var gap, lastError string
	var sourceErrorAt *time.Time
	var watermarkVersion int64
	if err = tx.QueryRowContext(ctx, `SELECT observed_through,config_version,coverage,gap_code,last_error_code,source_error_at FROM service_status_watermark WHERE id=1`).Scan(&s.ObservedThrough, &watermarkVersion, &coverageRaw, &gap, &lastError, &sourceErrorAt); err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	var coverage []service.ServiceStatusCoverage
	if json.Unmarshal(coverageRaw, &coverage) != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	end := now.Truncate(time.Minute)
	if s.ObservedThrough != nil {
		end = *s.ObservedThrough
	}
	s.Window = service.ServiceStatusInterval{Start: end.Add(-5 * time.Minute), End: end}
	if len(platforms) == 0 {
		s.Health, s.UnknownReason = "unknown", "no_enabled_platforms"
		if err = tx.Commit(); err != nil {
			return nil, service.ErrServiceStatusUnavailable
		}
		return s, nil
	}
	globalGap := gap
	switch {
	case s.ObservedThrough == nil:
		globalGap = "watermark_missing"
	case s.ObservedThrough.After(now):
		globalGap = "invalid_watermark"
	case now.Sub(*s.ObservedThrough) > 180*time.Second:
		globalGap = "watermark_stale"
	case watermarkVersion != cfg.Version:
		globalGap = "configuration_changed"
	case lastError != "":
		globalGap = "aggregation_error"
	case service.ServiceStatusSourceErrorSince(s.Window.Start) || (sourceErrorAt != nil && !sourceErrorAt.Before(s.Window.Start)):
		globalGap = "source_error"
	}
	for _, c := range coverage {
		if !containsStatusPlatform(platforms, c.Platform) || !c.End.After(s.HistoryRange.Start) || !c.Start.Before(s.HistoryRange.End) {
			continue
		}
		if c.Start.Before(s.HistoryRange.Start) {
			c.Start = s.HistoryRange.Start
		}
		if c.End.After(s.HistoryRange.End) {
			c.End = s.HistoryRange.End
		}
		s.Coverage = append(s.Coverage, c)
	}
	groups, err := loadStatusGroups(ctx, tx)
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	factStart := s.HistoryRange.Start
	if s.Window.Start.Before(factStart) {
		factStart = s.Window.Start
	}
	facts, err := loadStatusFacts(ctx, tx, platforms, factStart, now.Truncate(time.Minute))
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	s.Incidents, err = loadStatusIncidents(ctx, tx, platforms, s.HistoryRange.Start, s.HistoryRange.End, true)
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	open := map[statusScope]*service.ServiceStatusIncident{}
	for j := range s.Incidents {
		i := &s.Incidents[j]
		if i.ResolvedAt == nil {
			s.OpenIncidentCount++
			open[statusScope{i.Platform, i.GroupID, i.RequestedModel}] = i
		}
	}
	windows := map[statusScope]statusWindow{}
	states, err := loadStatusStates(ctx, tx, platforms)
	if err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	for scope := range states {
		windows[scope] = statusWindow{}
	}
	type historyKey struct {
		scope statusScope
		start time.Time
	}
	history := map[historyKey]statusWindow{}
	for _, f := range facts {
		if f.gap == "scope_unavailable" && !f.bucket.Before(s.Window.Start) && f.bucket.Before(s.Window.End) && globalGap == "" {
			globalGap = "scope_unavailable"
		}
		if f.scope.groupID == 0 || f.scope.platform == "" {
			if !f.bucket.Before(s.Window.Start) && f.bucket.Before(s.Window.End) {
				if globalGap == "" {
					globalGap = "scope_unavailable"
				}
			}
			continue
		}
		w := windows[f.scope]
		// 前一启用区间的事实只在历史显示，不能成为新启用期恢复样本。
		if !f.bucket.Before(s.Window.Start) && f.bucket.Before(s.Window.End) && !f.bucket.Before(cfg.epochs[f.scope.platform]) {
			addStatusCounts(&w.counts, f.counts)
			if f.lastQualified != nil {
				statusTimeMax(&w.qualified, *f.lastQualified)
			}
			if f.lastSuccess != nil {
				statusTimeMax(&w.success, *f.lastSuccess)
			}
			if f.gap != "" {
				w.gap = f.gap
			}
		}
		windows[f.scope] = w
		if !f.bucket.Before(s.HistoryRange.Start) && f.bucket.Before(s.HistoryRange.End) {
			bucket := s.HistoryRange.Start.Add((f.bucket.Sub(s.HistoryRange.Start) / step) * step)
			k := historyKey{f.scope, bucket}
			h := history[k]
			addStatusCounts(&h.counts, f.counts)
			if f.gap != "" {
				h.gap = f.gap
			}
			history[k] = h
		}
	}
	for scope := range open {
		if _, ok := windows[scope]; !ok {
			windows[scope] = statusWindow{}
		}
	}
	// 每个启用平台的 active 分组都保留等待请求的可见提示。
	for id, g := range groups {
		for _, p := range platforms {
			if g.platform != p && g.platform != "composite" {
				continue
			}
			found := false
			for scope := range windows {
				if scope.platform == p && scope.groupID == id {
					found = true
					break
				}
			}
			if !found {
				windows[statusScope{p, id, ""}] = statusWindow{}
			}
		}
	}
	for scope, w := range windows {
		leafGap := globalGap
		if leafGap == "" {
			leafGap = w.gap
		}
		if leafGap == "" && !statusCovered(coverage, scope.platform, s.Window.Start, s.Window.End) {
			leafGap = "coverage_gap"
		}
		metrics, health, reason := service.ServiceStatusEvaluate(w.counts, cfg.ServiceStatusConfig, leafGap)
		l := service.ServiceStatusLeaf{ServiceStatusScope: service.ServiceStatusScope{Platform: scope.platform, GroupID: scope.groupID, GroupName: groups[scope.groupID].name, RequestedModel: scope.model}, ServiceStatusMetrics: metrics, Health: health, UnknownReason: reason, LastEvidenceAt: w.qualified}
		if i := open[scope]; i != nil {
			l.IncidentID = &i.ID
			l.IncidentPhase = &i.Phase
			if health == "operational" {
				if i.Phase == "recovering" {
					l.Health = "recovering"
				} else {
					l.Health, l.UnknownReason = "unknown", "awaiting_data"
					l.ErrorRate, l.SuccessRate = nil, nil
				}
			}
		}
		s.Scopes = append(s.Scopes, l)
		for at := s.HistoryRange.Start; at.Before(s.HistoryRange.End); at = at.Add(step) {
			until := at.Add(step)
			if until.After(s.HistoryRange.End) {
				until = s.HistoryRange.End
			}
			h := history[historyKey{scope, at}]
			covered := statusCovered(coverage, scope.platform, at, until)
			hgap := h.gap
			if !covered && hgap == "" {
				hgap = "coverage_gap"
			}
			hm, _, hr := service.ServiceStatusEvaluate(h.counts, cfg.ServiceStatusConfig, hgap)
			s.History = append(s.History, service.ServiceStatusHistoryPoint{ServiceStatusScope: l.ServiceStatusScope, ServiceStatusMetrics: hm, ServiceStatusInterval: service.ServiceStatusInterval{Start: at, End: until}, Covered: covered, UnknownReason: hr})
		}
	}
	sort.Slice(s.Scopes, func(i, j int) bool {
		a, b := s.Scopes[i], s.Scopes[j]
		if a.Platform != b.Platform {
			return a.Platform < b.Platform
		}
		if a.GroupID != b.GroupID {
			return a.GroupID < b.GroupID
		}
		return a.RequestedModel < b.RequestedModel
	})
	sort.Slice(s.History, func(i, j int) bool {
		a, b := s.History[i], s.History[j]
		if a.Platform != b.Platform {
			return a.Platform < b.Platform
		}
		if a.GroupID != b.GroupID {
			return a.GroupID < b.GroupID
		}
		if a.RequestedModel != b.RequestedModel {
			return a.RequestedModel < b.RequestedModel
		}
		return a.Start.Before(b.Start)
	})
	summaryLeaves := append([]service.ServiceStatusLeaf{}, s.Scopes...)
	for _, p := range platforms {
		leaves := []service.ServiceStatusLeaf{}
		var counts service.ServiceStatusCounts
		for _, l := range s.Scopes {
			if l.Platform == p {
				leaves = append(leaves, l)
				addStatusCounts(&counts, l.ServiceStatusCounts)
			}
		}
		if len(leaves) == 0 {
			summaryLeaves = append(summaryLeaves, service.ServiceStatusLeaf{Health: "unknown", UnknownReason: "no_recent_requests"})
		}
		health, reason := service.ServiceStatusSummary(leaves)
		pgap := globalGap
		if pgap == "" {
			for _, l := range leaves {
				if l.UnknownReason != "" {
					pgap = l.UnknownReason
					break
				}
			}
		}
		metrics, _, _ := service.ServiceStatusEvaluate(counts, cfg.ServiceStatusConfig, pgap)
		s.Platforms = append(s.Platforms, service.ServiceStatusPlatform{Platform: p, Health: health, UnknownReason: reason, ServiceStatusMetrics: metrics})
	}
	s.Health, s.UnknownReason = service.ServiceStatusSummary(summaryLeaves)
	if globalGap != "" {
		s.Health, s.UnknownReason = "unknown", globalGap
		s.Gaps = append(s.Gaps, globalGap)
	}
	for _, p := range platforms {
		if !statusCovered(coverage, p, s.Window.Start, s.Window.End) {
			if !containsStatusPlatform(s.Gaps, "coverage_gap") {
				s.Gaps = append(s.Gaps, "coverage_gap")
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, service.ErrServiceStatusUnavailable
	}
	return s, nil
}
func containsStatusPlatform(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

type statusGroup struct{ name, platform string }

func loadStatusGroups(ctx context.Context, q statusQuerier) (map[int64]statusGroup, error) {
	rows, err := q.QueryContext(ctx, `SELECT id,name,platform FROM groups WHERE status='active' AND deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	groups := map[int64]statusGroup{}
	for rows.Next() {
		var id int64
		var g statusGroup
		if err = rows.Scan(&id, &g.name, &g.platform); err != nil {
			return nil, err
		}
		groups[id] = g
	}
	return groups, rows.Err()
}
