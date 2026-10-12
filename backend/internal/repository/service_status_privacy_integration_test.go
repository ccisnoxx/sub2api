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

func TestServiceStatusPrivacyRepositoryUsesFixedHistoryAndRealFacts(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	statusEnableSQL(t, now)
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `UPDATE service_status_config SET minimum_samples=1`)
	require.NoError(t, err)
	for _, row := range []struct {
		at      time.Time
		model   string
		samples int
	}{
		{now.Add(-time.Minute), "current-low-private-model", 4},
		{now.Add(-time.Minute), "visible-model", 5},
		{now.Truncate(time.Hour).Add(-30 * time.Minute), "history-low-private-model", 4},
		{now.Truncate(time.Hour).Add(-30 * time.Minute), "visible-model", 5},
	} {
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success,last_qualified_at,last_success_at) VALUES($1,'openai',$2,$3,$4,$1,$1)`, row.at, gid, row.model, row.samples)
		require.NoError(t, err)
	}
	coverage, err := json.Marshal([]service.ServiceStatusCoverage{{Platform: "openai", ServiceStatusInterval: service.ServiceStatusInterval{Start: now.Add(-24 * time.Hour), End: now}}})
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET observed_through=$1,coverage=$2`, now, string(coverage))
	require.NoError(t, err)
	s, err := r.Snapshot(ctx, "24h", "openai")
	require.NoError(t, err)
	require.Equal(t, now.Truncate(time.Hour), s.HistoryRange.End)
	require.Equal(t, s.HistoryRange.End.Add(-24*time.Hour), s.HistoryRange.Start)
	out := service.PrivateServiceStatusSnapshot(s)
	require.Equal(t, 5, out.PrivacyMinimumSamples)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "low-private-model")
	require.Contains(t, string(raw), "visible-model")
	require.Len(t, out.History, 1)
	require.Len(t, out.Scopes, 1)
	require.Nil(t, out.Platforms[0].Qualified)
	cfg, err := r.GetConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, cfg.MinimumSamples, "旧配置不能因隐私展示改写")
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM service_status_facts_1m WHERE group_id=$1`, gid).Scan(&count))
	require.Equal(t, 4, count, "展示裁剪不得删除真实事实")
}

func TestServiceStatusPrivacyRepositoryHiddenResolvedDoesNotEvictHistory(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	statusEnableSQL(t, now)
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `UPDATE service_status_config SET minimum_samples=1`)
	require.NoError(t, err)
	historyAt := now.Truncate(time.Hour).Add(-20 * time.Minute)
	for _, row := range []struct {
		at      time.Time
		model   string
		samples int
	}{{historyAt, "visible", 5}, {now.Add(-time.Minute), "visible", 5}, {now.Add(-time.Minute), "hidden", 1}} {
		_, err = integrationDB.ExecContext(ctx, `INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success,last_qualified_at,last_success_at) VALUES($1,'openai',$2,$3,$4,$1,$1)`, row.at, gid, row.model, row.samples)
		require.NoError(t, err)
	}
	coverage, err := json.Marshal([]service.ServiceStatusCoverage{{Platform: "openai", ServiceStatusInterval: service.ServiceStatusInterval{Start: now.Add(-24 * time.Hour), End: now}}})
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET observed_through=$1,coverage=$2`, now, string(coverage))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at,resolved_at,updates) SELECT gen_random_uuid(),'openai',$1,'visible','resolved',$2,$2,$2,$2,'[]' FROM generate_series(1,200)`, gid, historyAt)
	require.NoError(t, err)
	var hiddenID string
	// 检测在整点前、恢复在当前小时：即使测试恰好在整点执行也能覆盖隐藏候选占位。
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at,updates) VALUES(gen_random_uuid(),'openai',$1,'hidden','awaiting_data',$2,$2,$2,'[]') RETURNING id`, gid, now.Truncate(time.Hour).Add(-time.Minute)).Scan(&hiddenID))
	before, err := r.Snapshot(ctx, "24h", "openai")
	require.NoError(t, err)
	visibleBefore := service.PrivateServiceStatusSnapshot(before)
	require.Len(t, visibleBefore.Incidents, 200)
	_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_incidents SET phase='resolved',resolved_at=$1,last_evidence_at=$1 WHERE id=$2`, now, hiddenID)
	require.NoError(t, err)
	after, err := r.Snapshot(ctx, "24h", "openai")
	require.NoError(t, err)
	require.Len(t, after.Incidents, 200, "原始恢复事件上限保持不变")
	require.Contains(t, after.Incidents, service.ServiceStatusIncident{
		ServiceStatusScope: service.ServiceStatusScope{Platform: "openai", GroupID: gid, GroupName: after.Scopes[0].GroupName, RequestedModel: "hidden"},
		ID:                 hiddenID, Phase: "resolved", DetectedAt: now.Truncate(time.Hour).Add(-time.Minute), LastEvidenceAt: now, LastAbnormalAt: now.Truncate(time.Hour).Add(-time.Minute), ResolvedAt: &now, Updates: []service.ServiceStatusIncidentUpdate{},
	}, "底层读取仍包含当前小时真实恢复事件")
	visibleAfter := service.PrivateServiceStatusSnapshot(after)
	require.Len(t, visibleAfter.Incidents, 200, "隐藏事件不得挤掉可见历史事件")
	ids := func(incidents []service.ServiceStatusIncident) []string {
		result := make([]string, 0, len(incidents))
		for _, incident := range incidents {
			result = append(result, incident.ID)
		}
		return result
	}
	require.ElementsMatch(t, ids(visibleBefore.Incidents), ids(visibleAfter.Incidents))
}
