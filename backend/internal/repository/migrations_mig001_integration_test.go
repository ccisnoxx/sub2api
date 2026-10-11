//go:build integration

package repository

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// 旧 KIN 的 303 份 SQL 与目标的同名 SQL 逐字一致，差集经固定 SHA 清单核对。
// 使用真实 runner 和完整 SQL 集合，避免用手填账本或简化 schema 冒充升级。
func TestMIG001UpgradePaths(t *testing.T) {
	ctx := context.Background()
	nw, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nw.Remove(context.Background())) })
	pg, err := tcpostgres.Run(ctx, "postgres:18.6-alpine",
		tcpostgres.WithDatabase("mig001"), tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("mig001-isolated-test"), tcpostgres.BasicWaitStrategies(),
		network.WithNetwork([]string{"postgres"}, nw))
	if pg != nil {
		t.Cleanup(func() { require.NoError(t, pg.Terminate(context.Background())) })
	}
	require.NoError(t, err)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	oldKIN := mig001Files(t, false, false)
	oldPersonal := mig001Files(t, false, true)
	for _, initial := range []struct {
		name  string
		files fs.FS
		count int
	}{
		{"empty", nil, 0},
		{"old-kin", oldKIN, 303},
		{"already-personal", oldPersonal, 307},
	} {
		t.Run(initial.name, func(t *testing.T) {
			_, err := db.ExecContext(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
			require.NoError(t, err)
			if initial.files != nil {
				require.NoError(t, applyMigrationsFS(ctx, db, initial.files))
				mig001SeedUsage(t, db)
			}
			if initial.name == "already-personal" {
				_, err := db.ExecContext(ctx, `UPDATE service_status_config SET enabled=TRUE,version=7 WHERE id=1;
INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success) VALUES('2026-10-10T00:00:00Z','openai',1,'mig001-fixture',5)`)
				require.NoError(t, err)
			}
			before := mig001Ledger(t, db)
			require.Len(t, before, initial.count)
			var relationBefore int64
			if initial.files != nil {
				require.NoError(t, db.QueryRowContext(ctx, "SELECT pg_relation_filenode('public.usage_logs'::regclass)").Scan(&relationBefore))
			}
			started := time.Now()
			require.NoError(t, ApplyMigrations(ctx, db))
			t.Logf("升级耗时：%s；起始迁移数：%d", time.Since(started), initial.count)
			after := mig001Ledger(t, db)
			require.Len(t, after, 308)
			for name, row := range before {
				require.Equal(t, row, after[name], "旧 checksum / applied_at 必须保留：%s", name)
			}
			require.NoError(t, ApplyMigrations(ctx, db))
			require.Equal(t, after, mig001Ledger(t, db), "重复启动不得改写账本")
			if initial.files == nil {
				mig001SeedUsage(t, db)
			}
			mig001VerifyUsage(t, db)
			if initial.files != nil {
				var relationAfter int64
				require.NoError(t, db.QueryRowContext(ctx, "SELECT pg_relation_filenode('public.usage_logs'::regclass)").Scan(&relationAfter))
				require.NotZero(t, relationBefore)
				require.Equal(t, relationBefore, relationAfter, "新增观测列不得重写旧用量表")
			}
			var columnCount, tableCount int
			require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='public' AND ((table_name='usage_logs' AND column_name IN ('timing_version','strict_first_token_ms','last_token_ms','first_output_ms','first_output_kind','audio_output_tokens','completion_status','is_complete','usage_source','service_status_observation')) OR (table_name='ops_error_logs' AND column_name IN ('routing_diagnostics','service_status_observation')))`).Scan(&columnCount))
			require.Equal(t, 12, columnCount)
			require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('service_status_config','service_status_facts_1m','service_status_scope_states','service_status_incidents','service_status_watermark')`).Scan(&tableCount))
			require.Equal(t, 5, tableCount)
			if initial.name == "already-personal" {
				var enabled bool
				var version, successes int
				require.NoError(t, db.QueryRowContext(ctx, "SELECT enabled,version FROM service_status_config WHERE id=1").Scan(&enabled, &version))
				require.True(t, enabled)
				require.Equal(t, 7, version)
				require.NoError(t, db.QueryRowContext(ctx, "SELECT SUM(success) FROM service_status_facts_1m").Scan(&successes))
				require.Equal(t, 5, successes)
				mig001Restore(t, pg, db)
				mig001OldApplicationStarts(t, db, nw)
			}
			// 旧应用的 runner 字节与本 runner 相同；旧文件集不认识 242，也不得删其账本。
			require.NoError(t, applyMigrationsFS(ctx, db, oldPersonal))
			require.Equal(t, after, mig001Ledger(t, db))
			mig001SeedUsage(t, db)
			mig001VerifyUsage(t, db)
		})
	}

	t.Run("lock-failure-and-retry", func(t *testing.T) {
		_, err := db.ExecContext(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
		require.NoError(t, err)
		require.NoError(t, applyMigrationsFS(ctx, db, oldKIN))
		before := mig001Ledger(t, db)
		blocker, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = blocker.Rollback() }()
		_, err = blocker.ExecContext(ctx, "LOCK TABLE usage_logs IN ACCESS SHARE MODE")
		require.NoError(t, err)
		first := fstest.MapFS{}
		data, err := migrations.FS.ReadFile("251_add_usage_log_timing.sql")
		require.NoError(t, err)
		first["251_add_usage_log_timing.sql"] = &fstest.MapFile{Data: data}
		started := time.Now()
		err = applyMigrationsFS(ctx, db, first)
		require.ErrorContains(t, err, "lock timeout")
		t.Logf("251 锁等待失败耗时：%s", time.Since(started))
		require.Equal(t, before, mig001Ledger(t, db))
		var exists bool
		require.NoError(t, db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='usage_logs' AND column_name='timing_version')`).Scan(&exists))
		require.False(t, exists, "失败事务不得留下部分列")
		require.NoError(t, blocker.Rollback())
		require.NoError(t, ApplyMigrations(ctx, db))
		require.Len(t, mig001Ledger(t, db), 308)
	})

	t.Run("checksum-mismatch-fails-closed", func(t *testing.T) {
		_, err := db.ExecContext(ctx, "UPDATE schema_migrations SET checksum=$1 WHERE filename='252_add_ops_routing_diagnostics.sql'", strings.Repeat("0", 64))
		require.NoError(t, err)
		before := mig001Ledger(t, db)
		err = ApplyMigrations(ctx, db)
		require.ErrorContains(t, err, "migration 252_add_ops_routing_diagnostics.sql checksum mismatch")
		require.Equal(t, before, mig001Ledger(t, db), "错误身份不得被 runner 修补或放行")
	})
}

func mig001Files(t *testing.T, target242, personal bool) fstest.MapFS {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	files := fstest.MapFS{}
	for _, name := range names {
		if (!target242 && name == "242_drop_platform_check_constraints.sql") || (!personal && (strings.HasPrefix(name, "251_") || strings.HasPrefix(name, "252_") || strings.HasPrefix(name, "253_") || strings.HasPrefix(name, "254_"))) {
			continue
		}
		data, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		files[name] = &fstest.MapFile{Data: data}
	}
	return files
}

func mig001Ledger(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRow(`SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&exists))
	ledger := map[string]string{}
	if !exists {
		return ledger
	}
	rows, err := db.Query(`SELECT filename, checksum || '|' || to_char(applied_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US') FROM schema_migrations ORDER BY filename`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, value string
		require.NoError(t, rows.Scan(&name, &value))
		ledger[name] = value
	}
	require.NoError(t, rows.Err())
	return ledger
}

func mig001SeedUsage(t *testing.T, db *sql.DB) {
	t.Helper()
	result, err := db.Exec(`INSERT INTO users(email,password_hash) SELECT 'mig001@example.invalid','isolated-fixture' WHERE NOT EXISTS(SELECT 1 FROM users WHERE email='mig001@example.invalid');
INSERT INTO accounts(name,platform,type) SELECT 'mig001','openai','apikey' WHERE NOT EXISTS(SELECT 1 FROM accounts WHERE name='mig001');
INSERT INTO api_keys(user_id,key,name) SELECT id,'mig001-isolated-fixture','mig001' FROM users WHERE email='mig001@example.invalid' AND NOT EXISTS(SELECT 1 FROM api_keys WHERE key='mig001-isolated-fixture');
INSERT INTO usage_logs(user_id,api_key_id,account_id,model,input_tokens,output_tokens,actual_cost)
SELECT u.id,k.id,a.id,'mig001-fixture',11,7,0.125 FROM users u JOIN api_keys k ON k.user_id=u.id CROSS JOIN accounts a CROSS JOIN generate_series(1,10000)
WHERE u.email='mig001@example.invalid' AND a.name='mig001'`)
	require.NoError(t, err)
	rows, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 10000, rows, "夹具必须实际写入一万条合成用量")
}

func mig001VerifyUsage(t *testing.T, db *sql.DB) {
	t.Helper()
	var incorrect int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE model='mig001-fixture' AND (input_tokens<>11 OR output_tokens<>7 OR actual_cost<>0.125 OR timing_version<>0 OR completion_status<>'unknown' OR usage_source<>'unknown' OR strict_first_token_ms IS NOT NULL OR first_output_ms IS NOT NULL OR is_complete IS NOT NULL OR service_status_observation IS NOT NULL)`).Scan(&incorrect))
	require.Zero(t, incorrect, "旧显式列写入和历史行保留默认 unknown / NULL，不改变计费")
}

func mig001Restore(t *testing.T, pg *tcpostgres.PostgresContainer, db *sql.DB) {
	t.Helper()
	before := mig001Ledger(t, db)
	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM usage_logs").Scan(&count))
	code, output, err := pg.Exec(context.Background(), []string{"sh", "-ec", `pg_dump -U postgres -d mig001 -Fc -f /tmp/mig001.dump
createdb -U postgres mig001_restored
pg_restore -U postgres -d mig001_restored --exit-on-error /tmp/mig001.dump`})
	require.NoError(t, err)
	text, err := io.ReadAll(output)
	require.NoError(t, err)
	require.Zero(t, code, string(text))
	dsn, err := pg.ConnectionString(context.Background(), "sslmode=disable")
	require.NoError(t, err)
	restored, err := sql.Open("postgres", strings.Replace(dsn, "/mig001?", "/mig001_restored?", 1))
	require.NoError(t, err)
	defer func() { _ = restored.Close() }()
	require.Equal(t, before, mig001Ledger(t, restored), "恢复 checksum 和 applied_at")
	var restoredCount int
	require.NoError(t, restored.QueryRow("SELECT COUNT(*) FROM usage_logs").Scan(&restoredCount))
	require.Equal(t, count, restoredCount)
	mig001VerifyUsage(t, restored)
	var enabled bool
	var version, successes int
	require.NoError(t, restored.QueryRow("SELECT enabled,version FROM service_status_config WHERE id=1").Scan(&enabled, &version))
	require.True(t, enabled)
	require.Equal(t, 7, version)
	require.NoError(t, restored.QueryRow("SELECT SUM(success) FROM service_status_facts_1m").Scan(&successes))
	require.Equal(t, 5, successes)
	require.NoError(t, ApplyMigrations(context.Background(), restored))
}

// 固定当前生产镜像真实启动，验证扩展 schema / 额外账本不会阻止应用回退。
// 独立网络只配置合成数据库和 Redis，不接触生产服务或真实请求。
func mig001OldApplicationStarts(t *testing.T, db *sql.DB, nw *tc.DockerNetwork) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, "UPDATE service_status_config SET enabled=FALSE WHERE id=1")
	require.NoError(t, err)
	before := mig001Ledger(t, db)
	rdb, err := tc.Run(ctx, "redis:8.4-alpine", network.WithNetwork([]string{"redis"}, nw),
		tc.WithExposedPorts("6379/tcp"), tc.WithWaitStrategy(wait.ForListeningPort("6379/tcp")))
	if rdb != nil {
		t.Cleanup(func() { require.NoError(t, rdb.Terminate(context.Background())) })
	}
	require.NoError(t, err)
	app, err := tc.Run(ctx, "ghcr.io/ccisnoxx/sub2api@sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530",
		tc.WithImagePlatform("linux/amd64"), network.WithNetwork(nil, nw), tc.WithExposedPorts("8080/tcp"),
		tc.WithEnv(map[string]string{
			"AUTO_SETUP": "true", "DATABASE_HOST": "postgres", "DATABASE_PORT": "5432",
			"DATABASE_USER": "postgres", "DATABASE_PASSWORD": "mig001-isolated-test",
			"DATABASE_DBNAME": "mig001", "DATABASE_SSLMODE": "disable", "REDIS_HOST": "redis",
			"REDIS_PORT": "6379", "JWT_SECRET": strings.Repeat("m", 64),
			"TOTP_ENCRYPTION_KEY": strings.Repeat("a", 64), "TZ": "UTC",
			"ADMIN_EMAIL": "mig001-admin@example.invalid", "ADMIN_PASSWORD": "mig001-isolated-test",
		}), tc.WithWaitStrategy(wait.ForHTTP("/health").WithPort("8080/tcp").WithStartupTimeout(90*time.Second)))
	if app != nil {
		t.Cleanup(func() { require.NoError(t, app.Terminate(context.Background())) })
	}
	require.NoError(t, err, "固定旧应用必须在新 schema 上通过 /health")
	require.Equal(t, before, mig001Ledger(t, db), "旧应用启动保留扩展 schema 及账本")
	mig001VerifyUsage(t, db)
}
