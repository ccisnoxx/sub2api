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
