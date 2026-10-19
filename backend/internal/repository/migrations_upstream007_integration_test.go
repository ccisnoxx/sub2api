//go:build integration && upstream007

package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const upstream007OldApplicationImage = "ghcr.io/ccisnoxx/sub2api@sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530"

// 此入口必须显式选入，候选应是 Release dry-run 的真实 OCI 产物。
// 环境缺失或镜像身份不符直接失败，不能以 tag、重建镜像或旧 MIG-001 报告代替验收。
func TestUPSTREAM007UpgradePaths(t *testing.T) {
	candidate := upstream007CandidateImage(t)
	ctx := context.Background()
	nw, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nw.Remove(context.Background())) })

	for _, initial := range []struct {
		name     string
		files    fs.FS
		count    int
		personal bool
	}{
		{"empty", nil, 0, false},
		{"old-kin", mig001Files(t, false, false), 303, false},
		{"already-personal", mig001Files(t, false, true), 307, true},
		{"current-tps3", mig001Files(t, true, true), 308, true},
		{"current-klno4", mig001Klno4Files(t), 311, true},
	} {
		t.Run(initial.name, func(t *testing.T) {
			// 每条路径独立数据库和 Redis；复用 MIG-001 的数据库名称及恢复合同。
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

			var usage *upstream007Usage
			var state *upstream007ServiceState
			if initial.files != nil {
				require.NoError(t, applyMigrationsFS(ctx, db, initial.files))
				mig001SeedUsage(t, db)
				before := upstream007UsageSnapshot(t, db)
				require.Equal(t, 10000, before.count)
				usage = &before
			}
			if initial.personal {
				upstream007SeedServiceState(t, db)
				// 应用尚未创建；只改变 enabled，不走会递增版本或重置水位的配置 API。
				disabled := upstream007SetObservationEnabled(t, db, false)
				state = &disabled
			}
			beforeLedger := mig001Ledger(t, db)
			require.Len(t, beforeLedger, initial.count)

			rdb, err := tc.Run(ctx, "redis:8.4-alpine", network.WithNetwork([]string{"redis"}, nw),
				tc.WithExposedPorts("6379/tcp"), tc.WithWaitStrategy(wait.ForListeningPort("6379/tcp")))
			if rdb != nil {
				t.Cleanup(func() { require.NoError(t, rdb.Terminate(context.Background())) })
			}
			require.NoError(t, err)

			var upgradedLedger map[string]string
			// 先启动真实候选完成自动迁移；禁止提前运行旧应用补入 251–254。
			upstream007ApplicationCycle(t, "candidate-upgrade", candidate, nw, func() {
				ledger := upstream007VerifyLedger(t, db, beforeLedger)
				if upgradedLedger == nil {
					upgradedLedger = ledger
				} else {
					require.Equal(t, upgradedLedger, ledger, "候选停止时也不得改写账本")
				}
				if usage != nil {
					upstream007VerifyUsage(t, db, *usage)
				} else {
					require.Zero(t, upstream007UsageSnapshot(t, db).count)
				}
				if state != nil {
					require.Equal(t, *state, upstream007ServiceSnapshot(t, db))
				} else {
					// 空库、旧 KIN 由 254 默认禁用，直到首次候选完全停止才写状态夹具。
					initialState := upstream007ServiceSnapshot(t, db)
					require.False(t, initialState.enabled)
					state = &initialState
				}
			})

			if initial.files == nil {
				mig001SeedUsage(t, db)
				seeded := upstream007UsageSnapshot(t, db)
				usage = &seeded
			}
			if !initial.personal {
				upstream007SeedServiceState(t, db)
				disabled := upstream007SetObservationEnabled(t, db, false)
				state = &disabled
			}
			t.Log("观测已在应用启动前直接禁用；版本 7、既有事实和水位在切换期间必须保持")
			verify := func() {
				require.Equal(t, upgradedLedger, upstream007VerifyLedger(t, db, beforeLedger))
				upstream007VerifyUsage(t, db, *usage)
				require.Equal(t, *state, upstream007ServiceSnapshot(t, db), "五张状态表、版本及水位必须保留")
			}
			verify()
			// 仅用无池账本的合成数据检查启动兼容；禁止据此宣称生产数据可回退。
			upstream007ApplicationCycle(t, "fixed-old-startup-check", upstream007OldApplicationImage, nw, verify)
			upstream007ApplicationCycle(t, "candidate-return", candidate, nw, verify)

			// 此时三个应用容器均已停止。仅为既有 MIG-001 恢复 helper 的启用态合同
			// 恢复合成 enabled=true；版本、事实、水位和账本不随开关改变。
			restoreState := upstream007SetObservationEnabled(t, db, true)
			t.Log("所有应用停止后恢复合成启用态，复用 MIG-001 的实际 pg_dump/pg_restore helper")
			mig001Restore(t, pg, db)
			require.Equal(t, upgradedLedger, mig001Ledger(t, db))
			require.Equal(t, restoreState, upstream007ServiceSnapshot(t, db))
			upstream007VerifyUsage(t, db, *usage)

			// helper 已对恢复库再次运行真实 runner；补核完整状态和原始行指纹。
			require.True(t, strings.Contains(dsn, "/mig001?"), "恢复连接必须沿用 MIG-001 数据库命名合同")
			restored, err := sql.Open("postgres", strings.Replace(dsn, "/mig001?", "/mig001_restored?", 1))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, restored.Close()) })
			require.Equal(t, upgradedLedger, upstream007VerifyLedger(t, restored, beforeLedger), "恢复及重复迁移保留完整账本")
			upstream007VerifyUsage(t, restored, *usage)
			require.Equal(t, restoreState, upstream007ServiceSnapshot(t, restored), "恢复保留五张状态表、版本及水位")
		})
	}
}

func upstream007CandidateImage(t *testing.T) string {
	t.Helper()
	image := os.Getenv("UPSTREAM007_CANDIDATE_IMAGE")
	sha := os.Getenv("UPSTREAM007_CANDIDATE_SHA")
	require.NotEmpty(t, strings.TrimSpace(image), "显式选入 UPSTREAM-007 必须提供 UPSTREAM007_CANDIDATE_IMAGE")
	require.NotEmpty(t, sha, "显式选入 UPSTREAM-007 必须提供 UPSTREAM007_CANDIDATE_SHA")
	require.Regexp(t, `^[0-9a-fA-F]{40}$`, sha, "UPSTREAM007_CANDIDATE_SHA 必须为严格 40 位十六进制 Git SHA")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// 只读取身份字段，避免 docker inspect 输出 Config.Env 或连接配置。
	format := `{"id":{{json .Id}},"os":{{json .Os}},"architecture":{{json .Architecture}},"revision":{{json (index .Config.Labels "org.opencontainers.image.revision")}}}`
	output, err := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", format, "--", image).Output()
	require.NoError(t, err, "候选 OCI 必须已加载到当前 Docker daemon，禁止隐式重建或换用镜像")
	var inspected struct {
		ID           string `json:"id"`
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Revision     string `json:"revision"`
	}
	require.NoError(t, json.Unmarshal(output, &inspected))
	require.Equal(t, sha, inspected.Revision, "真实 OCI revision 必须等于候选源 SHA，不能把 tag 当身份")
	require.Equal(t, "linux", inspected.OS)
	require.Equal(t, "amd64", inspected.Architecture)
	require.Regexp(t, `^sha256:[0-9a-f]{64}$`, inspected.ID)
	t.Logf("候选源 SHA：%s；已加载 OCI image ID：%s", sha, inspected.ID)
	// 使用已检查的本地不可变 image ID，避免检查后 tag 被重新指向。
	return inspected.ID
}

// 检查健康后立即终止本阶段容器，下一阶段不与它共享应用生命周期。
// 失败也登记清理；启动、健康、停止错误均不能被吞掉。
func upstream007ApplicationCycle(t *testing.T, phase, image string, nw *tc.DockerNetwork, verify func()) {
	t.Helper()
	ctx := context.Background()
	app, err := tc.Run(ctx, image,
		tc.WithImagePlatform("linux/amd64"), network.WithNetwork(nil, nw), tc.WithExposedPorts("8080/tcp"),
		tc.WithEnv(map[string]string{
			"AUTO_SETUP": "true", "DATABASE_HOST": "postgres", "DATABASE_PORT": "5432",
			"DATABASE_USER": "postgres", "DATABASE_PASSWORD": "mig001-isolated-test",
			"DATABASE_DBNAME": "mig001", "DATABASE_SSLMODE": "disable", "REDIS_HOST": "redis",
			"REDIS_PORT": "6379", "JWT_SECRET": strings.Repeat("m", 64),
			"TOTP_ENCRYPTION_KEY": strings.Repeat("a", 64), "TZ": "UTC",
			"ADMIN_EMAIL": "mig001-admin@example.invalid", "ADMIN_PASSWORD": "mig001-isolated-test",
		}), tc.WithWaitStrategy(wait.ForHTTP("/health").WithPort("8080/tcp").WithStartupTimeout(90*time.Second)))
	terminated := false
	if app != nil {
		t.Cleanup(func() {
			if !terminated {
				require.NoError(t, app.Terminate(context.Background()), "失败阶段也必须清理应用容器")
			}
		})
	}
	require.NoError(t, err, "%s 必须真实启动并通过 /health", phase)
	verify()
	require.NoError(t, app.Terminate(ctx), "%s 必须完全停止后再进入下一阶段", phase)
	terminated = true
	verify()
	t.Logf("%s：健康启动、停止及数据核验完成", phase)
}

func upstream007VerifyLedger(t *testing.T, db *sql.DB, before map[string]string) map[string]string {
	t.Helper()
	ledger := mig001Ledger(t, db)
	require.Len(t, ledger, mig001TargetMigrationCount)
	for name, row := range before {
		require.Equal(t, row, ledger[name], "旧 checksum 和 applied_at 必须保留：%s", name)
	}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	require.Len(t, names, mig001TargetMigrationCount, "UPSTREAM-007 固定迁移集不能静默漂移")
	for _, name := range names {
		data, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		sum := sha256.Sum256([]byte(strings.TrimSpace(string(data))))
		require.True(t, strings.HasPrefix(ledger[name], hex.EncodeToString(sum[:])+"|"), "真实应用必须执行固定 SQL 身份及 checksum：%s", name)
	}
	return ledger
}

type upstream007Usage struct {
	count  int
	digest string
}

func upstream007UsageSnapshot(t *testing.T, db *sql.DB) upstream007Usage {
	t.Helper()
	// 移除仅由 251/253 追加的列后，升级前后比较全部原始行；新增列另查默认值。
	rows, err := db.Query(`SELECT (to_jsonb(u) - ARRAY['timing_version','strict_first_token_ms','last_token_ms','first_output_ms','first_output_kind','audio_output_tokens','completion_status','is_complete','usage_source','service_status_observation'])::text FROM usage_logs u WHERE model='mig001-fixture' ORDER BY id`)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	sum := sha256.New()
	count := 0
	for rows.Next() {
		var row string
		require.NoError(t, rows.Scan(&row))
		_, err := fmt.Fprintln(sum, row)
		require.NoError(t, err)
		count++
	}
	require.NoError(t, rows.Err())
	return upstream007Usage{count: count, digest: hex.EncodeToString(sum.Sum(nil))}
}

func upstream007VerifyUsage(t *testing.T, db *sql.DB, before upstream007Usage) {
	t.Helper()
	require.Equal(t, 10000, before.count, "必须实际保护一万条合成用量，不能用空集通过")
	require.Equal(t, before, upstream007UsageSnapshot(t, db), "全部原始用量行必须保留")
	mig001VerifyUsage(t, db)
	var incorrect int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE model='mig001-fixture' AND (last_token_ms IS NOT NULL OR first_output_kind IS NOT NULL OR audio_output_tokens IS NOT NULL)`).Scan(&incorrect))
	require.Zero(t, incorrect, "历史新增观测字段保持 NULL")
}

type upstream007ServiceState struct {
	enabled bool
	tables  map[string]string
}

func upstream007ServiceSnapshot(t *testing.T, db *sql.DB) upstream007ServiceState {
	t.Helper()
	state := upstream007ServiceState{tables: map[string]string{}}
	require.NoError(t, db.QueryRow("SELECT enabled FROM service_status_config WHERE id=1").Scan(&state.enabled))
	for _, name := range []string{"service_status_config", "service_status_facts_1m", "service_status_scope_states", "service_status_incidents", "service_status_watermark"} {
		projection := "to_jsonb(s)"
		if name == "service_status_config" {
			projection += " - 'enabled'"
		}
		// 表名只来自上述固定清单；配置开关独立记录，其他所有列按完整行比较。
		query := fmt.Sprintf("SELECT COALESCE(jsonb_agg(v ORDER BY v::text),'[]'::jsonb)::text FROM (SELECT %s AS v FROM %s s) rows", projection, name)
		var value string
		require.NoError(t, db.QueryRow(query).Scan(&value))
		state.tables[name] = value
	}
	return state
}

func upstream007SetObservationEnabled(t *testing.T, db *sql.DB, enabled bool) upstream007ServiceState {
	t.Helper()
	before := upstream007ServiceSnapshot(t, db)
	result, err := db.Exec("UPDATE service_status_config SET enabled=$1 WHERE id=1", enabled)
	require.NoError(t, err)
	count, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	before.enabled = enabled
	after := upstream007ServiceSnapshot(t, db)
	require.Equal(t, before, after, "直接禁用或恢复只改变 enabled，不得改变版本、事实和水位")
	return after
}

func upstream007SeedServiceState(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`UPDATE service_status_config SET enabled=TRUE,version=7 WHERE id=1;
INSERT INTO service_status_facts_1m(bucket_start,platform,group_id,requested_model,success) VALUES('2026-10-10T00:00:00Z','openai',1,'mig001-fixture',5);
INSERT INTO service_status_scope_states(platform,group_id,requested_model,last_observation_at,abnormal_count,config_version,observation_start)
VALUES('openai',1,'mig001-fixture','2026-10-10T00:01:00Z',1,7,'2026-10-10T00:00:00Z');
INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at)
VALUES('00000000-0000-4000-8000-000000000007','openai',1,'mig001-fixture','ongoing','2026-10-10T00:00:00Z','2026-10-10T00:01:00Z','2026-10-10T00:00:00Z');
UPDATE service_status_watermark SET observed_through='2026-10-10T00:01:00Z',config_version=7 WHERE id=1`)
	require.NoError(t, err)
	var version, successes int
	require.NoError(t, db.QueryRow("SELECT version FROM service_status_config WHERE id=1").Scan(&version))
	require.Equal(t, 7, version)
	require.NoError(t, db.QueryRow("SELECT SUM(success) FROM service_status_facts_1m").Scan(&successes))
	require.Equal(t, 5, successes)
}
