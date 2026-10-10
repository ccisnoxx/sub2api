//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func statusSQLFixture(t *testing.T) (*serviceStatusRepository, int64, time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `TRUNCATE service_status_facts_1m,service_status_scope_states,service_status_incidents; UPDATE service_status_watermark SET observed_through=NULL,config_version=1,coverage='[]',gap_code='',last_error_code='',source_error_at=NULL; UPDATE service_status_config SET version=1,enabled=false,platforms='["openai"]',minimum_samples=5,warning_error_rate=.05,outage_error_rate=.9,abnormal_windows=2,recovery_windows=3,observation_starts='{}'`)
	require.NoError(t, err)
	var gid int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform) VALUES($1,'openai') RETURNING id`, "status-"+uuid.NewString()).Scan(&gid))
	var now time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT date_trunc('minute',clock_timestamp())`).Scan(&now))
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM ops_error_logs WHERE group_id=$1`, gid)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, gid)
		require.NoError(t, err)
	})
	return &serviceStatusRepository{db: integrationDB}, gid, now.UTC()
}
func statusEnableSQL(t *testing.T, now time.Time) {
	epochs, _ := json.Marshal(map[string]time.Time{"openai": now.Add(-30 * time.Minute)})
	_, err := integrationDB.ExecContext(context.Background(), `UPDATE service_status_config SET enabled=true,observation_starts=$1`, string(epochs))
	require.NoError(t, err)
}
func statusInsertOps(t *testing.T, gid int64, at time.Time, n int, kind, reason string) {
	t.Helper()
	for j := 0; j < n; j++ {
		r := statusRow(uuid.NewString(), kind, reason, at)
		_, err := integrationDB.ExecContext(context.Background(), `INSERT INTO ops_error_logs(group_id,platform,model,requested_model,error_phase,error_type,created_at,service_status_observation) VALUES($1,'openai','gpt','gpt','upstream','upstream_error',$2,$3)`, gid, at, string(r.metadata))
		require.NoError(t, err)
	}
}

// 固定在已完成分钟的旧源，复现共享库全站缺口；只清理本次插入的行。
func statusInsertUnscopedLegacyOps(t *testing.T, at time.Time) {
	t.Helper()
	ctx := context.Background()
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO ops_error_logs(model,error_phase,error_type,created_at) VALUES('legacy-ambient','upstream','test',$1) RETURNING id`, at).Scan(&id))
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM ops_error_logs WHERE id=$1`, id)
		require.NoError(t, err)
	})
}
func statusInsertSuccess(t *testing.T, gid int64, at time.Time, n int) {
	t.Helper()
	ctx := context.Background()
	user, key, account := committedUsageLogTestResources(t, service.PlatformOpenAI)
	for j := 0; j < n; j++ {
		r := statusRow(uuid.NewString(), "completed", "", at)
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,group_id,request_id,model,requested_model,completion_status,is_complete,actual_cost,created_at,service_status_observation) VALUES($1,$2,$3,$4,$5,'gpt','gpt','completed',true,0,$6,$7)`, user.ID, key.ID, account.ID, gid, uuid.NewString(), at, string(r.metadata))
		require.NoError(t, err)
	}
}
func statusIncidentPhaseSQL(t *testing.T) (string, int) {
	t.Helper()
	var phase string
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT phase FROM service_status_incidents ORDER BY detected_at DESC LIMIT 1`).Scan(&phase))
	require.NoError(t, integrationDB.QueryRow(`SELECT recovery_count FROM service_status_scope_states LIMIT 1`).Scan(&count))
	return phase, count
}
func TestServiceStatusRepositoryConfigEpochVersionAndDisabled(t *testing.T) {
	r, _, now := statusSQLFixture(t)
	ctx := context.Background()
	c, err := r.GetConfig(ctx)
	require.NoError(t, err)
	require.False(t, c.Enabled)
	require.Equal(t, []string{"openai"}, c.Platforms)
	_, err = r.Snapshot(ctx, "24h", "")
	require.ErrorIs(t, err, service.ErrServiceStatusDisabled)
	c.Enabled = true
	updated, err := r.UpdateConfig(ctx, c)
	require.NoError(t, err)
	require.Equal(t, int64(2), updated.Version)
	_, err = r.UpdateConfig(ctx, c)
	require.ErrorIs(t, err, service.ErrServiceStatusConfigConflict)
	var raw []byte
	require.NoError(t, integrationDB.QueryRow(`SELECT observation_starts FROM service_status_config`).Scan(&raw))
	var epochs map[string]time.Time
	require.NoError(t, json.Unmarshal(raw, &epochs))
	require.Equal(t, now.Add(time.Minute), epochs["openai"])
	updated.Platforms = []string{}
	_, err = r.UpdateConfig(ctx, updated)
	require.NoError(t, err)
	s, err := r.Snapshot(ctx, "24h", "")
	require.NoError(t, err)
	require.Equal(t, "no_enabled_platforms", s.UnknownReason)
}
func TestServiceStatusRepositoryAtomicEventsRepeatAndRecovery(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	statusEnableSQL(t, now)
	ctx := context.Background()
	statusInsertOps(t, gid, now.Add(-12*time.Minute), 5, "upstream_error", "provider_5xx")
	require.NoError(t, r.Aggregate(ctx, now.Add(-11*time.Minute)))
	require.NoError(t, r.Aggregate(ctx, now.Add(-11*time.Minute)))
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM service_status_incidents`).Scan(&count))
	require.Zero(t, count)
	statusInsertOps(t, gid, now.Add(-11*time.Minute), 5, "upstream_error", "provider_5xx")
	require.NoError(t, r.Aggregate(ctx, now.Add(-10*time.Minute)))
	phase, _ := statusIncidentPhaseSQL(t)
	require.Equal(t, "detected", phase)
	// 故障旧样本移出窗口；没有新请求不能恢复。
	require.NoError(t, r.Aggregate(ctx, now.Add(-5*time.Minute)))
	phase, _ = statusIncidentPhaseSQL(t)
	require.Equal(t, "awaiting_data", phase)
	for j := 0; j < 3; j++ {
		at := now.Add(time.Duration(j-5) * time.Minute)
		statusInsertSuccess(t, gid, at, 5)
		end := at.Add(time.Minute)
		require.NoError(t, r.Aggregate(ctx, end))
		require.NoError(t, r.Aggregate(ctx, end))
		phase, recovery := statusIncidentPhaseSQL(t)
		if j < 2 {
			require.Equal(t, "recovering", phase)
			require.Equal(t, j+1, recovery)
		} else {
			require.Equal(t, "resolved", phase)
		}
	}
	s, err := r.Snapshot(ctx, "24h", "openai")
	require.NoError(t, err)
	require.Zero(t, s.OpenIncidentCount)
	require.Equal(t, "operational", s.Health)
	require.Len(t, s.Incidents, 1)
	var success, failure int64
	require.NoError(t, integrationDB.QueryRow(`SELECT SUM(success),SUM(failure) FROM service_status_facts_1m WHERE group_id=$1`, gid).Scan(&success, &failure))
	require.Equal(t, int64(15), success)
	require.Equal(t, int64(10), failure)
	// 同一个物理请求的零费用成功与同步 Ops completed 不会重复计数。
	var raw []byte
	var at time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT service_status_observation,created_at FROM usage_logs WHERE group_id=$1 LIMIT 1`, gid).Scan(&raw, &at))
	_, err = integrationDB.Exec(`INSERT INTO ops_error_logs(group_id,model,requested_model,error_phase,error_type,created_at,service_status_observation) VALUES($1,'gpt','gpt','upstream','recovered',$2,$3)`, gid, at, string(raw))
	require.NoError(t, err)
	require.NoError(t, r.Aggregate(ctx, now.Add(-2*time.Minute)))
	require.NoError(t, integrationDB.QueryRow(`SELECT SUM(success) FROM service_status_facts_1m WHERE group_id=$1`, gid).Scan(&success))
	require.Equal(t, int64(15), success)
}
func TestServiceStatusRepositoryLimitRollsBackAndLeaderConfigSerializes(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	statusEnableSQL(t, now)
	statusInsertUnscopedLegacyOps(t, now.Add(-time.Minute+30*time.Second))
	ctx := context.Background()
	require.NoError(t, r.Aggregate(ctx, now))
	var through time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT observed_through FROM service_status_watermark`).Scan(&through))
	const factsQuery = `SELECT COALESCE(jsonb_agg(to_jsonb(f) ORDER BY bucket_start,platform,group_id,requested_model),'[]'::jsonb)::text FROM service_status_facts_1m f`
	var factsBefore string
	require.NoError(t, integrationDB.QueryRow(factsQuery).Scan(&factsBefore))
	require.NotEqual(t, "[]", factsBefore, "受控旧源必须使扫描前已有事实")
	_, err := integrationDB.Exec(`INSERT INTO ops_error_logs(group_id,model,error_phase,error_type,created_at) SELECT $1,'overflow','upstream','test',$2 FROM generate_series(1,50001)`, gid, now.Add(-time.Minute))
	require.NoError(t, err)
	err = r.Aggregate(ctx, now)
	require.ErrorIs(t, err, service.ErrServiceStatusObservationLimit)
	var after time.Time
	var code string
	require.NoError(t, integrationDB.QueryRow(`SELECT observed_through,last_error_code FROM service_status_watermark`).Scan(&after, &code))
	require.Equal(t, through, after)
	require.Equal(t, "SERVICE_STATUS_OBSERVATION_LIMIT", code)
	var factsAfter string
	require.NoError(t, integrationDB.QueryRow(factsQuery).Scan(&factsAfter))
	require.Equal(t, factsBefore, factsAfter, "失败聚合必须完整保留已提交事实")
	_, err = integrationDB.Exec(`DELETE FROM ops_error_logs WHERE group_id=$1`, gid)
	require.NoError(t, err)
	lock, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	var held bool
	require.NoError(t, lock.QueryRow(`SELECT pg_try_advisory_xact_lock($1)`, statusLockID).Scan(&held))
	require.True(t, held)
	short, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	require.NoError(t, r.Aggregate(short, now))
	require.NoError(t, lock.Rollback())
	rowlock, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	var v int64
	require.NoError(t, rowlock.QueryRow(`SELECT version FROM service_status_config WHERE id=1 FOR UPDATE`).Scan(&v))
	c, err := r.GetConfig(ctx)
	require.NoError(t, err)
	c.WarningErrorRate = .1
	done := make(chan error, 1)
	go func() { _, e := r.UpdateConfig(ctx, c); done <- e }()
	select {
	case err = <-done:
		t.Fatalf("配置更新越过聚合行锁: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	require.NoError(t, rowlock.Commit())
	require.NoError(t, <-done)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for j := 0; j < 2; j++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- r.Aggregate(ctx, now) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
}
func TestServiceStatusRepositoryTTLFilterLimitAndPersistentSourceGap(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	statusEnableSQL(t, now)
	ctx := context.Background()
	_, err := integrationDB.Exec(`INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success) VALUES($1,'openai',$2,'old',5)`, now.Add(-32*24*time.Hour), gid)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at,resolved_at,updates) SELECT gen_random_uuid(),'openai',$1,'history','resolved',$2,$2,$2,$3,'[]' FROM generate_series(1,201)`, gid, now.Add(-2*time.Hour), now.Add(-time.Hour))
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at,updates) SELECT gen_random_uuid(),'openai',$1,'open-'||n,'awaiting_data',$2,$2,$2,'[]' FROM generate_series(1,205) n`, gid, now.Add(-32*24*time.Hour))
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE service_status_config SET enabled=false`)
	require.NoError(t, err)
	require.NoError(t, r.Aggregate(ctx, now))
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM service_status_facts_1m`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM service_status_incidents WHERE resolved_at IS NULL`).Scan(&count))
	require.Equal(t, 205, count)
	_, err = integrationDB.Exec(`UPDATE service_status_config SET enabled=true`)
	require.NoError(t, err)
	s, err := r.Snapshot(ctx, "24h", "openai")
	require.NoError(t, err)
	require.Equal(t, 205, s.OpenIncidentCount)
	require.Len(t, s.Incidents, 405)
	s, err = r.Snapshot(ctx, "24h", "anthropic")
	require.NoError(t, err)
	require.Empty(t, s.Incidents)
	require.Zero(t, s.OpenIncidentCount)
	require.NoError(t, r.RecordSourceError(ctx, now.Add(-time.Minute)))
	require.NoError(t, r.Aggregate(ctx, now))
	s, err = r.Snapshot(ctx, "24h", "openai")
	require.NoError(t, err)
	require.Equal(t, "source_error", s.UnknownReason)
	require.Contains(t, s.Gaps, "source_error")
	// 新实例只读取持久缺口，不依赖本地 producer 回调。
	fresh := NewServiceStatusRepository(integrationDB)
	s, err = fresh.Snapshot(ctx, "24h", "openai")
	require.NoError(t, err)
	require.Equal(t, "source_error", s.UnknownReason)
}

func TestServiceStatusRepositoryMissingModelIsVisibleGlobalGap(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	statusEnableSQL(t, now)
	at := now.Add(-time.Minute)
	row := statusRow(uuid.NewString(), "upstream_error", "provider_5xx", at)
	_, err := integrationDB.Exec(`INSERT INTO ops_error_logs(group_id,model,requested_model,error_phase,error_type,created_at,service_status_observation) VALUES($1,'','','upstream','upstream_error',$2,$3)`, gid, at, string(row.metadata))
	require.NoError(t, err)
	require.NoError(t, r.Aggregate(context.Background(), now))
	s, err := r.Snapshot(context.Background(), "24h", "openai")
	require.NoError(t, err)
	require.Equal(t, "scope_unavailable", s.UnknownReason)
	require.Contains(t, s.Gaps, "scope_unavailable")
	require.Len(t, s.Scopes, 1)
	require.Equal(t, "unknown", s.Scopes[0].RequestedModel)
	require.Nil(t, s.Scopes[0].ErrorRate)
}

func TestServiceStatusRepositoryGroupVisibilityRestartsRecovery(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	ctx := context.Background()
	statusEnableSQL(t, now)
	id := uuid.NewString()
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_scope_states(platform,group_id,requested_model,last_evidence_at,last_observation_at,abnormal_count,recovery_count,config_version,observation_start,last_health) VALUES('openai',$1,'gpt',$2,$2,0,2,1,$3,'operational');`, gid, now.Add(-time.Minute), now.Add(-30*time.Minute))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at) VALUES($1,'openai',$2,'gpt','recovering',$3,$4,$3)`, id, gid, now.Add(-10*time.Minute), now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, gid)
	require.NoError(t, err)
	phase, count := statusIncidentPhaseSQL(t)
	require.Equal(t, "awaiting_data", phase)
	require.Zero(t, count, "组失去可见性必须立即清零连续恢复")
	require.NoError(t, r.Aggregate(ctx, now.Add(time.Minute)))
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status='active' WHERE id=$1`, gid)
	require.NoError(t, err)
	var visibleAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT last_evidence_at FROM service_status_scope_states WHERE group_id=$1`, gid).Scan(&visibleAt))
	// 保留其他测试的历史源；从重新可见基线推进完整十分钟重算区间后再提供新恢复证据。
	recoveryStart := visibleAt.UTC().Truncate(time.Minute).Add(10 * time.Minute)
	for j := 1; j <= 3; j++ {
		at := recoveryStart.Add(time.Duration(j-1)*time.Minute + time.Second)
		statusInsertSuccess(t, gid, at, 5)
		require.NoError(t, r.Aggregate(ctx, at.Add(time.Minute)))
		phase, count = statusIncidentPhaseSQL(t)
		if j < 3 {
			require.Equal(t, "recovering", phase)
			require.Equal(t, j, count)
		} else {
			require.Equal(t, "resolved", phase)
		}
	}
}
func TestServiceStatusRepositoryEmptyEnabledPlatformIsUnknown(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	ctx := context.Background()
	// 隔离测试库既有 active/composite 分组，保证第二个平台确实没有叶子。
	rows, err := integrationDB.QueryContext(ctx, `UPDATE groups SET status='inactive' WHERE id<>$1 AND status='active' RETURNING id`, gid)
	require.NoError(t, err)
	var otherIDs []int64
	for rows.Next() {
		var id int64
		require.NoError(t, rows.Scan(&id))
		otherIDs = append(otherIDs, id)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	t.Cleanup(func() {
		for _, id := range otherIDs {
			_, _ = integrationDB.ExecContext(ctx, `UPDATE groups SET status='active' WHERE id=$1`, id)
		}
	})
	statusEnableSQL(t, now)
	epochs, _ := json.Marshal(map[string]time.Time{"openai": now.Add(-30 * time.Minute), "anthropic": now.Add(-30 * time.Minute)})
	_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_config SET platforms='["openai","anthropic"]',observation_starts=$1`, string(epochs))
	require.NoError(t, err)
	statusInsertSuccess(t, gid, now.Add(-time.Minute), 5)
	require.NoError(t, r.Aggregate(ctx, now))
	snap, err := r.Snapshot(ctx, "24h", "")
	require.NoError(t, err)
	for _, leaf := range snap.Scopes {
		require.NotEqual(t, "anthropic", leaf.Platform, "反例必须没有 Anthropic 叶子")
	}
	require.Equal(t, "unknown", snap.Health, "没有请求的已启用平台不能被另一个平台的成功掩盖")
	var found bool
	for _, p := range snap.Platforms {
		if p.Platform == "anthropic" {
			found = true
			require.Equal(t, "unknown", p.Health)
		}
	}
	require.True(t, found)
}

func TestServiceStatusRepositoryFrozenIncidentHasNullRates(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	ctx := context.Background()
	statusEnableSQL(t, now)
	statusInsertUnscopedLegacyOps(t, now.Add(-time.Minute+30*time.Second))
	statusInsertSuccess(t, gid, now.Add(-time.Minute), 4)
	statusInsertOps(t, gid, now.Add(-time.Minute), 1, "upstream_error", "provider_5xx")
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_scope_states(platform,group_id,requested_model,last_evidence_at,last_observation_at,abnormal_count,recovery_count,config_version,observation_start,last_health) VALUES('openai',$1,'gpt',$2,$2,2,0,1,$3,'degraded')`, gid, now.Add(-time.Minute), now.Add(-30*time.Minute))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at) VALUES($1,'openai',$2,'gpt','ongoing',$3,$4,$4)`, uuid.NewString(), gid, now.Add(-10*time.Minute), now.Add(-time.Minute))
	require.NoError(t, err)
	cfg, err := r.GetConfig(ctx)
	require.NoError(t, err)
	cfg.WarningErrorRate = .5
	_, err = r.UpdateConfig(ctx, cfg)
	require.NoError(t, err)
	// 在聚合之前检查配置 owner 的立即冻结，避免全站缺口替它完成冻结而掩盖回归。
	phase, recovery := statusIncidentPhaseSQL(t)
	require.Equal(t, "awaiting_data", phase)
	require.Zero(t, recovery)
	require.NoError(t, r.Aggregate(ctx, now))
	snap, err := r.Snapshot(ctx, "24h", "openai")
	require.NoError(t, err)
	require.Contains(t, snap.Gaps, "scope_unavailable", "旧源的全站缺口不得被事件冻结掩盖")
	found := false
	for _, leaf := range snap.Scopes {
		if leaf.GroupID == gid && leaf.RequestedModel == "gpt" {
			found = true
			require.Equal(t, "unknown", leaf.Health)
			require.Equal(t, "scope_unavailable", leaf.UnknownReason)
			require.NotNil(t, leaf.IncidentPhase)
			require.Equal(t, "awaiting_data", *leaf.IncidentPhase)
			require.Nil(t, leaf.ErrorRate)
			require.Nil(t, leaf.SuccessRate)
		}
	}
	require.True(t, found)
}
func TestServiceStatusRepositoryVisibilityLockAndBaseline(t *testing.T) {
	t.Run("shared_freeze_lock", func(t *testing.T) {
		_, gid, _ := statusSQLFixture(t)
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		var other int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform) VALUES($1,'openai') RETURNING id`, uuid.NewString()).Scan(&other))
		t.Cleanup(func() {
			_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM groups WHERE id=$1`, other)
		})
		a, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer a.Rollback()
		_, err = a.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, gid)
		require.NoError(t, err)
		b, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer b.Rollback()
		var locked int64
		require.NoError(t, b.QueryRowContext(ctx, `SELECT id FROM groups WHERE id=$1 FOR UPDATE`, other).Scan(&locked))
		changed := make(chan error, 1)
		go func() {
			_, err := b.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, other)
			changed <- err
		}()
		select {
		case err := <-changed:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("不同组冻结不应争用独占配置锁")
		}
		require.NoError(t, b.Commit())
		_, err = a.ExecContext(ctx, `UPDATE groups SET status='active' WHERE id=$1`, other)
		require.NoError(t, err)
		require.NoError(t, a.Commit())
	})
	t.Run("baseline_after_lock", func(t *testing.T) {
		_, gid, now := statusSQLFixture(t)
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_scope_states(platform,group_id,requested_model,last_evidence_at,last_observation_at,config_version,observation_start) VALUES('openai',$1,'gpt',$2,$2,1,$3)`, gid, now.Add(-time.Minute), now.Add(-30*time.Minute))
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status='inactive' WHERE id=$1`, gid)
		require.NoError(t, err)
		blocker, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer blocker.Rollback()
		var version int64
		require.NoError(t, blocker.QueryRowContext(ctx, `SELECT version FROM service_status_config WHERE id=1 FOR UPDATE`).Scan(&version))
		conn, err := integrationDB.Conn(ctx)
		require.NoError(t, err)
		defer conn.Close()
		var pid int
		require.NoError(t, conn.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
		done := make(chan error, 1)
		go func() {
			_, err := conn.ExecContext(ctx, `UPDATE groups SET status='active' WHERE id=$1`, gid)
			done <- err
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
			return err == nil && waiting
		}, time.Second, time.Millisecond*5)
		var releasedAt time.Time
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&releasedAt))
		require.NoError(t, blocker.Commit())
		require.NoError(t, <-done)
		var baseline time.Time
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT last_evidence_at FROM service_status_scope_states WHERE group_id=$1`, gid).Scan(&baseline))
		require.False(t, baseline.Before(releasedAt), "新基线必须在可见性事务获锁后形成")
	})
}
