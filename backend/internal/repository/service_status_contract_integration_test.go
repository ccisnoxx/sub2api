//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 经真实仓储快照和 JSON 边界固化健康、缺口和停用合同。
func TestServiceStatusSnapshotStateContract(t *testing.T) {
	for _, tc := range []struct {
		name, health, reason, global, factGap, phase string
		counts                                       service.ServiceStatusCounts
		disabled, noPlatforms, uncovered             bool
	}{
		{name: "零请求", health: "unknown", reason: "no_recent_requests"},
		{name: "全排除", health: "unknown", reason: "no_qualified_requests", counts: service.ServiceStatusCounts{Excluded: 5}},
		{name: "全未知", health: "unknown", reason: "terminal_unknown", counts: service.ServiceStatusCounts{Unknown: 5}},
		{name: "低样本", health: "unknown", reason: "insufficient_samples", counts: service.ServiceStatusCounts{Success: 4}},
		{name: "最低样本", health: "operational", counts: service.ServiceStatusCounts{Success: 5}},
		{name: "warning边界", health: "degraded", counts: service.ServiceStatusCounts{Success: 19, Failure: 1}},
		{name: "outage边界", health: "outage", counts: service.ServiceStatusCounts{Success: 1, Failure: 9}},
		{name: "恢复窗口未满", health: "recovering", phase: "recovering", counts: service.ServiceStatusCounts{Success: 5}},
		{name: "等待新证据", health: "unknown", reason: "awaiting_data", phase: "awaiting_data", counts: service.ServiceStatusCounts{Success: 5}},
		{name: "检测事故不能直接恢复", health: "unknown", reason: "awaiting_data", phase: "detected", counts: service.ServiceStatusCounts{Success: 5}},
		{name: "持续事故不能直接恢复", health: "unknown", reason: "awaiting_data", phase: "ongoing", counts: service.ServiceStatusCounts{Success: 5}},
		{name: "停用", disabled: true},
		{name: "无启用平台", noPlatforms: true, health: "unknown", reason: "no_enabled_platforms"},
		{name: "覆盖缺口", uncovered: true, health: "unknown", reason: "coverage_gap", counts: service.ServiceStatusCounts{Success: 5}},
		{name: "水位缺失", global: "watermark_missing", health: "unknown", reason: "watermark_missing"},
		{name: "未来水位", global: "invalid_watermark", health: "unknown", reason: "invalid_watermark"},
		{name: "过期水位", global: "watermark_stale", health: "unknown", reason: "watermark_stale"},
		{name: "配置版本变更", global: "configuration_changed", health: "unknown", reason: "configuration_changed"},
		{name: "聚合失败", global: "aggregation_error", health: "unknown", reason: "aggregation_error"},
		{name: "源错误", global: "source_error", health: "unknown", reason: "source_error", counts: service.ServiceStatusCounts{Success: 5}},
		{name: "范围归属缺失", factGap: "scope_unavailable", health: "unknown", reason: "scope_unavailable", counts: service.ServiceStatusCounts{Success: 5}},
		{name: "缺少关联", factGap: "linkage_unavailable", health: "unknown", reason: "linkage_unavailable", counts: service.ServiceStatusCounts{Unknown: 5}},
		{name: "缺少终态", factGap: "terminal_missing", health: "unknown", reason: "terminal_missing", counts: service.ServiceStatusCounts{Unknown: 5}},
		{name: "终态冲突", factGap: "terminal_conflict", health: "unknown", reason: "terminal_conflict", counts: service.ServiceStatusCounts{Unknown: 5}},
		{name: "入口不支持", factGap: "unsupported_entry", health: "unknown", reason: "unsupported_entry", counts: service.ServiceStatusCounts{Unknown: 5}},
		{name: "无法分类", factGap: "unclassified", health: "unknown", reason: "unclassified", counts: service.ServiceStatusCounts{Unknown: 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, gid, now := statusSQLFixture(t)
			ctx := context.Background()
			if !tc.disabled {
				statusEnableSQL(t, now)
			}
			if tc.noPlatforms {
				_, err := integrationDB.ExecContext(ctx, `UPDATE service_status_config SET platforms='[]'`)
				require.NoError(t, err)
			}
			coverage, err := json.Marshal([]service.ServiceStatusCoverage{{Platform: "openai", ServiceStatusInterval: service.ServiceStatusInterval{Start: now.Add(-30 * time.Hour), End: now}}})
			require.NoError(t, err)
			if tc.uncovered {
				coverage = []byte(`[]`)
			}
			_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET observed_through=$1,coverage=$2`, now, string(coverage))
			require.NoError(t, err)
			switch tc.global {
			case "watermark_missing":
				_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET observed_through=NULL`)
			case "invalid_watermark":
				_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET observed_through=$1`, now.Add(2*time.Minute))
			case "watermark_stale":
				_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET observed_through=$1`, now.Add(-4*time.Minute))
			case "configuration_changed":
				_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET config_version=0`)
			case "aggregation_error":
				_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET last_error_code='test_failure'`)
			case "source_error":
				err = r.RecordSourceError(ctx, now.Add(-time.Minute))
			}
			require.NoError(t, err)
			var qualifiedAt, successAt *time.Time
			at := now.Add(-time.Second)
			if tc.counts.Success+tc.counts.Failure > 0 {
				qualifiedAt = &at
			}
			if tc.counts.Success > 0 {
				successAt = &at
			}
			_, err = integrationDB.ExecContext(ctx, `INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success,failure,excluded,unknown,last_qualified_at,last_success_at,gap_code) VALUES($1,'openai',$2,'gpt',$3,$4,$5,$6,$7,$8,$9)`, now.Add(-time.Minute), gid, tc.counts.Success, tc.counts.Failure, tc.counts.Excluded, tc.counts.Unknown, qualifiedAt, successAt, tc.factGap)
			require.NoError(t, err)
			if tc.phase != "" {
				_, err = integrationDB.ExecContext(ctx, `INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at,updates) VALUES(gen_random_uuid(),'openai',$1,'gpt',$2,$3,$3,$3,'[]')`, gid, tc.phase, now.Add(-time.Minute))
				require.NoError(t, err)
			}
			snapshot, err := r.Snapshot(ctx, "24h", "openai")
			if tc.disabled {
				require.ErrorIs(t, err, service.ErrServiceStatusDisabled)
				require.Nil(t, snapshot)
				return
			}
			require.NoError(t, err)
			require.True(t, snapshot.MonitoringEnabled)
			require.Equal(t, tc.health, snapshot.Health)
			require.Contains(t, snapshot.BoundedObservationNotice, "本部署已观察请求")
			require.Contains(t, snapshot.BoundedObservationNotice, "不代表供应商全局可用性或 SLA")
			if tc.noPlatforms {
				require.Equal(t, tc.reason, snapshot.UnknownReason)
				require.Empty(t, snapshot.Platforms)
				require.Empty(t, snapshot.Scopes)
				return
			}
			if tc.global != "" {
				require.Equal(t, tc.reason, snapshot.UnknownReason)
				require.Contains(t, snapshot.Gaps, tc.reason)
			} else if tc.factGap == "scope_unavailable" {
				require.Equal(t, tc.reason, snapshot.UnknownReason)
				require.Contains(t, snapshot.Gaps, tc.reason)
			} else if tc.health == "unknown" {
				require.Equal(t, "awaiting_data", snapshot.UnknownReason)
			} else {
				require.Empty(t, snapshot.UnknownReason)
			}
			require.Len(t, snapshot.Scopes, 1)
			leaf := snapshot.Scopes[0]
			require.Equal(t, tc.health, leaf.Health)
			require.Equal(t, tc.reason, leaf.UnknownReason)
			require.Len(t, snapshot.Platforms, 1)
			require.Equal(t, tc.health, snapshot.Platforms[0].Health)
			if tc.phase != "" {
				require.Equal(t, &tc.phase, leaf.IncidentPhase)
				require.Equal(t, 1, snapshot.OpenIncidentCount)
				require.Nil(t, snapshot.Incidents[0].ResolvedAt)
			}
			if tc.health == "unknown" {
				require.Nil(t, leaf.SuccessRate)
				require.Nil(t, leaf.ErrorRate)
				require.Nil(t, snapshot.Platforms[0].SuccessRate)
				require.Nil(t, snapshot.Platforms[0].ErrorRate)
			} else {
				require.NotNil(t, leaf.SuccessRate)
				require.NotNil(t, leaf.ErrorRate)
				require.InDelta(t, float64(tc.counts.Failure)/float64(tc.counts.Success+tc.counts.Failure), *leaf.ErrorRate, 1e-12)
			}
			for _, point := range snapshot.History {
				if point.UnknownReason != "" || !point.Covered {
					require.Nil(t, point.SuccessRate)
					require.Nil(t, point.ErrorRate)
				}
			}
			raw, err := json.Marshal(leaf)
			require.NoError(t, err)
			if tc.health == "unknown" {
				require.Contains(t, string(raw), `"success_rate":null`)
				require.Contains(t, string(raw), `"error_rate":null`)
			}
		})
	}
}
