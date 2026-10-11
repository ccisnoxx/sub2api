//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestServiceStatusMonitorSQLCapacityAndLogicalDailyRows(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	ctx := context.Background()
	day := now.UTC().Truncate(24 * time.Hour)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success)
 VALUES($1,'openai',$4,'a',5),($2,'openai',$4,'a',5),($3,'openai',$4,'a',5)`, day.Add(time.Minute), day.Add(-time.Minute), day.Add(-25*time.Hour), gid)
	require.NoError(t, err)
	c, err := readStatusCapacity(ctx, integrationDB, now)
	require.NoError(t, err)
	require.Equal(t, int64(1), c.TodayRows)
	require.Equal(t, int64(1), c.YesterdayRows)
	require.Positive(t, c.IndexBytes)
	require.Equal(t, c.TableBytes+c.IndexBytes, c.TotalBytes)
	require.True(t, c.ActivityComplete)
	// 两次真实聚合重写同一事实，日行数不受物理 INSERT 次数影响。
	statusEnableSQL(t, now)
	statusInsertOps(t, gid, now.Add(-time.Minute), 5, "upstream_error", "provider_5xx")
	require.NoError(t, r.Aggregate(ctx, now))
	first, err := readStatusCapacity(ctx, integrationDB, now)
	require.NoError(t, err)
	require.NoError(t, r.Aggregate(ctx, now))
	second, err := readStatusCapacity(ctx, integrationDB, now)
	require.NoError(t, err)
	require.Equal(t, first.TodayRows, second.TodayRows)
	require.Equal(t, first.YesterdayRows, second.YesterdayRows)
}

func TestServiceStatusMonitorSQLUninheritedStatsRoleIsIncomplete(t *testing.T) {
	_, _, now := statusSQLFixture(t)
	ctx := context.Background()
	role := "status_monitor_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := integrationDB.ExecContext(ctx, `CREATE ROLE `+role+` NOINHERIT; GRANT pg_read_all_stats TO `+role+`; GRANT SELECT ON service_status_facts_1m TO `+role)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DROP OWNED BY `+role+`; DROP ROLE `+role)
		require.NoError(t, err)
	})
	conn, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_, err = conn.ExecContext(ctx, `SET ROLE `+role)
	require.NoError(t, err)
	defer func() {
		_, err := conn.ExecContext(ctx, `RESET ROLE`)
		require.NoError(t, err)
	}()
	var member, usable bool
	require.NoError(t, conn.QueryRowContext(ctx, `SELECT pg_has_role(current_user,'pg_read_all_stats','MEMBER'),pg_has_role(current_user,'pg_read_all_stats','USAGE')`).Scan(&member, &usable))
	require.True(t, member)
	require.False(t, usable)
	c, err := readStatusCapacity(ctx, conn, now)
	require.NoError(t, err)
	require.False(t, c.ActivityComplete, "成员身份不代表会话当前有权读取其他角色的事务详情")
}

func TestServiceStatusMonitorSQLRollbackAndCleanupFailure(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	c := captureStatusLogs(t)
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model) VALUES($1,'openai',$2,'expired')`, now.Add(-32*24*time.Hour), gid)
	require.NoError(t, err)
	// 用真实表锁造成清理超时，验证零提交删除、失败计数和固定错误码。
	blocker, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.ExecContext(ctx, `LOCK TABLE service_status_facts_1m IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	timeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, r.Aggregate(timeout, now), service.ErrServiceStatusUnavailable)
	require.NoError(t, blocker.Rollback())
	e := c.last(t, "service_status_cleanup")
	require.Equal(t, "rolled_back", e.Fields["outcome"])
	require.Equal(t, int64(0), e.Fields["facts_deleted"])
	require.Equal(t, "aggregation_timeout", c.last(t, "service_status_aggregate").Fields["alert_code"])
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM service_status_facts_1m WHERE requested_model='expired'`).Scan(&count))
	require.Equal(t, 1, count)
	var through *time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT observed_through FROM service_status_watermark`).Scan(&through))
	require.Nil(t, through)
	require.NoError(t, r.Aggregate(ctx, now))
	require.Equal(t, int64(1), c.last(t, "service_status_cleanup").Fields["facts_deleted"])
	require.Equal(t, int64(0), c.last(t, "service_status_aggregate").Fields["consecutive_failures"])
}

func TestServiceStatusMonitorSQLLaterFailureDoesNotReportDeletedRows(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	c := captureStatusLogs(t)
	ctx := context.Background()
	statusEnableSQL(t, now)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model) VALUES($1,'openai',$2,'expired');`, now.Add(-32*24*time.Hour), gid)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_watermark SET observed_through=$1`, now.Add(time.Minute))
	require.NoError(t, err)
	require.ErrorIs(t, r.Aggregate(ctx, now), service.ErrServiceStatusUnavailable)
	e := c.last(t, "service_status_cleanup")
	require.Equal(t, "rolled_back", e.Fields["outcome"])
	require.Equal(t, "watermark", e.Fields["failed_stage"])
	require.Equal(t, int64(1), e.Fields["facts_delete_attempted"])
	require.Equal(t, int64(0), e.Fields["facts_deleted"])
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM service_status_facts_1m WHERE requested_model='expired'`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestServiceStatusMonitorSQLProjectedRetentionCompletesWithinCycle(t *testing.T) {
	r, gid, now := statusSQLFixture(t)
	ctx := context.Background()
	statusEnableSQL(t, now)
	// 真实部署已观察峰值为每分钟三个范围：按全天连续峰值生成 31 天容量。
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success)
 SELECT bucket,'openai',$2,'capacity-'||model,1 FROM generate_series($1::timestamptz-interval '31 days',$1::timestamptz-interval '11 minutes',interval '1 minute') bucket CROSS JOIN generate_series(1,3) model`, now, gid)
	require.NoError(t, err)
	var before int64
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM service_status_facts_1m`).Scan(&before))
	require.Greater(t, before, int64(133000))
	statusInsertOps(t, gid, now.Add(-time.Minute), 14, "upstream_error", "provider_5xx")
	started := time.Now()
	require.NoError(t, r.Aggregate(ctx, now))
	elapsed := time.Since(started)
	require.Less(t, elapsed, 45*time.Second)
	c, err := readStatusCapacity(ctx, integrationDB, now)
	require.NoError(t, err)
	t.Logf("合成连续峰值：facts=%d aggregate_with_probe_ms=%d table_bytes=%d index_bytes=%d", before, elapsed.Milliseconds(), c.TableBytes, c.IndexBytes)
}
