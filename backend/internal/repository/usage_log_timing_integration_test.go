//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbusagelog "github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// 真实 SQL/批量写入需要已提交依赖；按本 fixture 的 ID 清理，避免污染全站统计。
func committedUsageLogTestResources(t *testing.T, platform string) (*service.User, *service.APIKey, *service.Account) {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "timing-" + uuid.NewString() + "@example.com"})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
		require.NoError(t, err)
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-timing-" + uuid.NewString(), Name: "k"})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE api_key_id=$1`, apiKey.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM api_keys WHERE id=$1`, apiKey.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM auth_cache_invalidation_outbox WHERE cache_key=encode(sha256(convert_to($1,'UTF8')),'hex')`, apiKey.Key)
		require.NoError(t, err)
	})
	account := mustCreateAccount(t, client, &service.Account{Name: "timing-" + uuid.NewString(), Platform: platform})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
		require.NoError(t, err)
	})
	return user, apiKey, account
}

func timingTestLog(t *testing.T, timing service.UsageTiming) *service.UsageLog {
	t.Helper()
	user, apiKey, account := committedUsageLogTestResources(t, service.PlatformAnthropic)
	legacy, duration := 77, 640
	return &service.UsageLog{
		UsageTiming: timing, UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: uuid.NewString(), Model: "gpt-5", FirstTokenMs: &legacy, DurationMs: &duration,
		InputTokens: 40, OutputTokens: 30, TotalCost: 0.75, ActualCost: 0.6, RateMultiplier: 0.8,
		CreatedAt: time.Now().UTC(),
	}
}

func observedTestTiming() service.UsageTiming {
	first, last, output, audio := 15, 510, 9, 7
	kind, complete := "audio", true
	return service.UsageTiming{
		TimingVersion: 1, StrictFirstTokenMs: &first, LastTokenMs: &last,
		FirstOutputMs: &output, FirstOutputKind: &kind, AudioOutputTokens: &audio,
		CompletionStatus: service.CompletionStatusCompleted, IsComplete: &complete,
		UsageSource: service.UsageSourceUpstreamFinal,
	}
}

func requireTimingLogRoundTrip(t *testing.T, repo *usageLogRepository, log *service.UsageLog) {
	t.Helper()
	got, err := repo.GetByID(context.Background(), log.ID)
	require.NoError(t, err)
	require.Equal(t, log.CanonicalUsageTiming(), got.UsageTiming)
	require.Equal(t, log.FirstTokenMs, got.FirstTokenMs, "旧首字保持原值")
	require.Equal(t, log.DurationMs, got.DurationMs)
	require.Equal(t, log.InputTokens, got.InputTokens)
	require.Equal(t, log.OutputTokens, got.OutputTokens)
	require.Equal(t, log.TotalCost, got.TotalCost)
	require.Equal(t, log.ActualCost, got.ActualCost)
	require.Equal(t, log.RateMultiplier, got.RateMultiplier)
}

func TestUsageLogTiming_SingleAndDuplicatePersistence(t *testing.T) {
	ctx := context.Background()
	repo := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB)
	log := timingTestLog(t, observedTestTiming())
	inserted, err := repo.createSingle(ctx, integrationDB, log)
	require.NoError(t, err)
	require.True(t, inserted)
	requireTimingLogRoundTrip(t, repo, log)

	duplicate := *log
	duplicate.UsageTiming = service.UsageTiming{
		TimingVersion: 1, CompletionStatus: service.CompletionStatusUpstreamError,
		UsageSource: service.UsageSourceUpstreamPartial,
	}
	duplicate.OutputTokens, duplicate.TotalCost = 99, 9
	inserted, err = repo.createSingle(ctx, integrationDB, &duplicate)
	require.NoError(t, err)
	require.False(t, inserted)
	require.Equal(t, log.ID, duplicate.ID)
	requireTimingLogRoundTrip(t, repo, log)
}

func TestUsageLogTiming_BatchPersistenceAndDeduplication(t *testing.T) {
	repo := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB)
	observed := timingTestLog(t, observedTestTiming())
	unknown := *observed
	unknown.RequestID = uuid.NewString()
	unknown.UsageTiming = service.UsageTiming{}
	duplicate := *observed
	duplicate.UsageTiming = service.UsageTiming{CompletionStatus: service.CompletionStatusInterrupted}
	logs := []*service.UsageLog{observed, &unknown, &duplicate}
	batch := make([]usageLogCreateRequest, 0, len(logs))
	for _, log := range logs {
		batch = append(batch, usageLogCreateRequest{log: log, prepared: prepareUsageLogInsert(log), resultCh: make(chan usageLogCreateResult, 1)})
	}
	repo.flushCreateBatch(integrationDB, batch)
	for i, req := range batch {
		result := <-req.resultCh
		require.NoError(t, result.err)
		require.Equal(t, i < 2, result.inserted)
	}
	require.Equal(t, observed.ID, duplicate.ID)
	requireTimingLogRoundTrip(t, repo, observed)
	requireTimingLogRoundTrip(t, repo, &unknown)

	// 再次通过实际排队入口写同一主键，不覆盖已观察的快照。
	inserted, err := repo.Create(context.Background(), &duplicate)
	require.NoError(t, err)
	require.False(t, inserted)
	requireTimingLogRoundTrip(t, repo, observed)
	listed, _, err := repo.ListByUser(context.Background(), observed.UserID, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, listed, 2)
	for _, row := range listed {
		if row.ID == observed.ID {
			require.Equal(t, observed.UsageTiming, row.UsageTiming)
		} else {
			require.Equal(t, unknown.CanonicalUsageTiming(), row.UsageTiming)
		}
	}
}

func TestUsageLogTiming_BestEffortAndFallbackPersistence(t *testing.T) {
	ctx := context.Background()
	repo := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB)
	log := timingTestLog(t, observedTestTiming())
	require.NoError(t, repo.CreateBestEffort(ctx, log))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", log.RequestID, log.APIKeyID).Scan(&log.ID))
	requireTimingLogRoundTrip(t, repo, log)

	duplicate := *log
	duplicate.UsageTiming = service.UsageTiming{CompletionStatus: service.CompletionStatusInterrupted}
	// 新 repository 绕过最近记录缓存，实际 SQL ON CONFLICT 分支仍保留原行。
	otherRepo := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB)
	require.NoError(t, otherRepo.CreateBestEffort(ctx, &duplicate))
	requireTimingLogRoundTrip(t, repo, log)

	good := *log
	good.RequestID, good.ID = uuid.NewString(), 0
	partial, audio, kind := false, 3, "audio"
	good.IsComplete, good.AudioOutputTokens, good.FirstOutputKind = &partial, &audio, &kind
	good.CompletionStatus, good.UsageSource = service.CompletionStatusClientDisconnected, service.UsageSourceUpstreamPartial
	bad := good
	bad.RequestID, bad.AccountID = uuid.NewString(), -1 // 外键错误使整批失败。
	batch := []usageLogBestEffortRequest{
		{prepared: prepareUsageLogInsert(&good), apiKeyID: good.APIKeyID, resultCh: make(chan error, 1)},
		{prepared: prepareUsageLogInsert(&bad), apiKeyID: bad.APIKeyID, resultCh: make(chan error, 1)},
	}
	repo.flushBestEffortBatch(integrationDB, batch)
	require.NoError(t, <-batch[0].resultCh, "有效行经单条 fallback 落库")
	require.Error(t, <-batch[1].resultCh, "无效行保留显式错误")
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", good.RequestID, good.APIKeyID).Scan(&good.ID))
	requireTimingLogRoundTrip(t, repo, &good)
	var badCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs WHERE request_id = $1", bad.RequestID).Scan(&badCount))
	require.Zero(t, badCount)
}

func TestUsageLogTiming_EntPersistence(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	log := timingTestLog(t, observedTestTiming())
	row, err := client.UsageLog.Create().
		SetUserID(log.UserID).SetAPIKeyID(log.APIKeyID).SetAccountID(log.AccountID).
		SetRequestID(log.RequestID).SetModel(log.Model).SetFirstTokenMs(*log.FirstTokenMs).
		SetTimingVersion(log.TimingVersion).SetStrictFirstTokenMs(*log.StrictFirstTokenMs).
		SetLastTokenMs(*log.LastTokenMs).SetFirstOutputMs(*log.FirstOutputMs).
		SetFirstOutputKind(*log.FirstOutputKind).SetAudioOutputTokens(*log.AudioOutputTokens).
		SetCompletionStatus(log.CompletionStatus).SetIsComplete(*log.IsComplete).
		SetUsageSource(log.UsageSource).Save(ctx)
	require.NoError(t, err)
	got, err := client.UsageLog.Query().Where(dbusagelog.IDEQ(row.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, log.TimingVersion, got.TimingVersion)
	require.Equal(t, log.StrictFirstTokenMs, got.StrictFirstTokenMs)
	require.Equal(t, log.LastTokenMs, got.LastTokenMs)
	require.Equal(t, log.FirstOutputMs, got.FirstOutputMs)
	require.Equal(t, log.FirstOutputKind, got.FirstOutputKind)
	require.Equal(t, log.AudioOutputTokens, got.AudioOutputTokens)
	require.Equal(t, log.CompletionStatus, got.CompletionStatus)
	require.Equal(t, log.IsComplete, got.IsComplete)
	require.Equal(t, log.UsageSource, got.UsageSource)

	historical, err := client.UsageLog.Create().
		SetUserID(log.UserID).SetAPIKeyID(log.APIKeyID).SetAccountID(log.AccountID).
		SetRequestID(uuid.NewString()).SetModel(log.Model).Save(ctx)
	require.NoError(t, err)
	stored, err := newUsageLogRepositoryWithSQL(client, integrationDB).GetByID(ctx, historical.ID)
	require.NoError(t, err)
	require.Equal(t, (&service.UsageLog{}).CanonicalUsageTiming(), stored.UsageTiming)
}

func TestMigration251_UsageTimingHistoricalAndIdempotent(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	// 临时表遮蔽 public.usage_logs，真实执行迁移且不影响其他测试中的全量表。
	_, err := tx.ExecContext(ctx, "CREATE TEMP TABLE usage_logs (id BIGINT, first_token_ms INTEGER)")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "INSERT INTO usage_logs (id, first_token_ms) VALUES (1, 123)")
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("251_add_usage_log_timing.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err, "同一增量迁移可安全重执行")
	// 扩展后旧写入者省略所有新列，仍写入未知值。
	_, err = tx.ExecContext(ctx, "INSERT INTO usage_logs (id, first_token_ms) VALUES (2, 456)")
	require.NoError(t, err)
	for id, legacy := range map[int]int{1: 123, 2: 456} {
		var version int16
		var old int
		var first, last, output, audio sql.NullInt64
		var kind sql.NullString
		var status, source string
		var complete sql.NullBool
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT first_token_ms, timing_version, strict_first_token_ms, last_token_ms,
first_output_ms, first_output_kind, audio_output_tokens, completion_status, is_complete, usage_source FROM usage_logs WHERE id = $1`, id).
			Scan(&old, &version, &first, &last, &output, &kind, &audio, &status, &complete, &source))
		require.Equal(t, legacy, old)
		require.Zero(t, version)
		require.False(t, first.Valid)
		require.False(t, last.Valid)
		require.False(t, output.Valid)
		require.False(t, kind.Valid)
		require.False(t, audio.Valid)
		require.False(t, complete.Valid)
		require.Equal(t, "unknown", status)
		require.Equal(t, "unknown", source)
	}
}
