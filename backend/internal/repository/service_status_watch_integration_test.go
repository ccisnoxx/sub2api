//go:build integration

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 读取检查器维护的实际 SQL/判定函数，避免测试另写一份探测逻辑。
func TestServiceStatusWatchSQLStoppedSourceAndReenable(t *testing.T) {
	_, _, now := statusSQLFixture(t)
	statusEnableSQL(t, now)
	ctx := context.Background()
	script := "../../../tools/service-status-watch.py"
	query, err := exec.Command("python3", "-c", "import runpy,sys; print(runpy.run_path(sys.argv[1])['WATCH_SQL'])", script).Output()
	require.NoError(t, err)
	probe := func() map[string]any {
		t.Helper()
		tx, err := integrationDB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		var raw []byte
		require.NoError(t, tx.QueryRowContext(ctx, string(query)).Scan(&raw))
		require.NoError(t, tx.Commit())
		cmd := exec.Command("python3", "-c", "import runpy,sys,json; print(json.dumps(runpy.run_path(sys.argv[1])['evaluate'](json.load(sys.stdin))))", script)
		cmd.Stdin = bytes.NewReader(raw)
		result, err := cmd.Output()
		require.NoError(t, err)
		var event map[string]any
		require.NoError(t, json.Unmarshal(result, &event))
		return event
	}
	update := func(query string, args ...any) {
		t.Helper()
		_, err := integrationDB.ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}
	require.Equal(t, []any{"watermark_missing"}, probe()["alert_codes"])
	update(`UPDATE service_status_watermark SET observed_through=$1`, now.Add(-4*time.Minute))
	require.Equal(t, []any{"watermark_stale"}, probe()["alert_codes"])
	update(`UPDATE service_status_watermark SET observed_through=$1,source_error_at=clock_timestamp()`, now)
	require.Equal(t, []any{"source_error"}, probe()["alert_codes"])
	// 源缺口时点过期后，已提交的 gap 仍须报警；不能被新鲜水位掩盖。
	update(`UPDATE service_status_watermark SET source_error_at=$1,gap_code='source_error'`, now.Add(-11*time.Minute))
	require.Equal(t, []any{"source_error"}, probe()["alert_codes"])
	update(`UPDATE service_status_watermark SET gap_code=''`)
	require.Empty(t, probe()["alert_codes"])
	update(`UPDATE service_status_config SET enabled=false`)
	update(`UPDATE service_status_watermark SET observed_through=$1,last_error_code='SERVICE_STATUS_AGGREGATION_FAILED'`, now.Add(-4*time.Minute))
	require.Equal(t, "disabled", probe()["outcome"])
	// 使用配置 owner 重新启用，旧水位和旧缺口不能触发启动期停滞误报。
	r := &serviceStatusRepository{db: integrationDB}
	cfg, err := r.GetConfig(ctx)
	require.NoError(t, err)
	cfg.Enabled = true
	_, err = r.UpdateConfig(ctx, cfg)
	require.NoError(t, err)
	update(`UPDATE service_status_watermark SET last_error_code=''`)
	require.Empty(t, probe()["alert_codes"])
	update(`UPDATE service_status_watermark SET observed_through=clock_timestamp()+interval '1 minute'`)
	require.Equal(t, []any{"invalid_watermark"}, probe()["alert_codes"])
}
