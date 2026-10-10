//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	dbusagelog "github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func sourceTestObservation(turn int) *service.ServiceStatusObservation {
	ctx := service.WithServiceStatusRequest(context.Background(), turn)
	service.SetServiceStatusAttemptFailure(ctx, service.CompletionStatusCompleted, "", time.Now().UTC())
	return service.ServiceStatusFinalObservation(ctx, "")
}

func TestServiceStatusSourcePersistenceUsageSingleBatchBestEffortAndFallback(t *testing.T) {
	ctx := context.Background()
	repo := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB)
	timing := observedTestTiming()
	timing.ServiceStatusObservation = sourceTestObservation(0)
	one := timingTestLog(t, timing)
	inserted, err := repo.Create(ctx, one)
	require.NoError(t, err)
	require.True(t, inserted)
	requireTimingLogRoundTrip(t, repo, one)
	// 原计费幂等约束继续生效，不覆盖旧源事实。
	duplicate := *one
	duplicate.UsageTiming = one.UsageTiming.Clone()
	duplicate.ServiceStatusObservation.ReasonCode = nil
	inserted, err = repo.Create(ctx, &duplicate)
	require.NoError(t, err)
	require.False(t, inserted)
	requireTimingLogRoundTrip(t, repo, one)
	entRow, err := testEntClient(t).UsageLog.Query().Where(dbusagelog.IDEQ(one.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, one.ServiceStatusObservation.ObservationKey, entRow.ServiceStatusObservation["observation_key"])

	historical := timingTestLog(t, service.UsageTiming{})
	observed := timingTestLog(t, observedTestTiming())
	observed.ServiceStatusObservation = sourceTestObservation(2)
	batch := []usageLogCreateRequest{{log: historical, prepared: prepareUsageLogInsert(historical), resultCh: make(chan usageLogCreateResult, 1)}, {log: observed, prepared: prepareUsageLogInsert(observed), resultCh: make(chan usageLogCreateResult, 1)}}
	repo.flushCreateBatch(integrationDB, batch)
	for _, req := range batch {
		result := <-req.resultCh
		require.NoError(t, result.err)
		require.True(t, result.inserted)
		requireTimingLogRoundTrip(t, repo, req.log)
	}
	best := timingTestLog(t, observedTestTiming())
	best.ServiceStatusObservation = sourceTestObservation(3)
	require.NoError(t, repo.CreateBestEffort(ctx, best))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2", best.RequestID, best.APIKeyID).Scan(&best.ID))
	requireTimingLogRoundTrip(t, repo, best)

	good := timingTestLog(t, observedTestTiming())
	good.ServiceStatusObservation = sourceTestObservation(4)
	bad := *good
	bad.RequestID = uuid.NewString()
	bad.AccountID = -1
	fallback := []usageLogBestEffortRequest{{prepared: prepareUsageLogInsert(good), apiKeyID: good.APIKeyID, resultCh: make(chan error, 1)}, {prepared: prepareUsageLogInsert(&bad), apiKeyID: bad.APIKeyID, resultCh: make(chan error, 1)}}
	repo.flushBestEffortBatch(integrationDB, fallback)
	require.NoError(t, <-fallback[0].resultCh)
	require.Error(t, <-fallback[1].resultCh)
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2", good.RequestID, good.APIKeyID).Scan(&good.ID))
	requireTimingLogRoundTrip(t, repo, good)
}

func TestServiceStatusSourcePersistenceOpsSingleBatchAndInternalPrivacy(t *testing.T) {
	ctx := context.Background()
	repo := NewOpsRepository(integrationDB)
	ops := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	terminal := sourceTestObservation(1)
	attempt := service.ServiceStatusAttemptObservation(service.WithServiceStatusRequest(ctx, 0), "provider_capacity", time.Now().UTC())
	entries := []*service.OpsInsertErrorLogInput{
		{RequestID: uuid.NewString(), StatusCode: 502, Severity: "error", ErrorType: "upstream_error", ErrorPhase: "upstream", ErrorOwner: "provider", ErrorSource: "upstream_http", ServiceStatusObservation: terminal},
		{RequestID: uuid.NewString(), StatusCode: 200, Severity: "warning", ErrorType: "upstream_error", ErrorPhase: "upstream", ErrorOwner: "provider", ErrorSource: "upstream_http", ServiceStatusObservation: attempt},
	}
	// 实际BatchInsert路径另用两条，避免RecordErrorBatch的单条快捷路径代替batch证据。
	third := *entries[0]
	third.RequestID = uuid.NewString()
	fourth := *entries[1]
	fourth.RequestID = uuid.NewString()
	for _, entry := range append(entries, &third, &fourth) {
		t.Cleanup(func() {
			_, err := integrationDB.ExecContext(ctx, `DELETE FROM ops_error_logs WHERE request_id=$1`, entry.RequestID)
			require.NoError(t, err)
		})
	}
	require.NoError(t, ops.RecordError(ctx, entries[0]))
	require.NoError(t, ops.RecordErrorBatch(ctx, entries[1:]))
	require.NoError(t, ops.RecordErrorBatch(ctx, []*service.OpsInsertErrorLogInput{&third, &fourth}))
	for _, entry := range append(entries, &third, &fourth) {
		var id int64
		var raw string
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id, service_status_observation::text FROM ops_error_logs WHERE request_id=$1", entry.RequestID).Scan(&id, &raw))
		got, err := service.DecodeServiceStatusObservation([]byte(raw))
		require.NoError(t, err)
		require.Equal(t, entry.ServiceStatusObservation, got)
		detail, err := repo.GetErrorLogByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, got, detail.ServiceStatusObservation)
		dto, err := json.Marshal(detail)
		require.NoError(t, err)
		require.NotContains(t, string(dto), "observation_key")
	}
}

func TestServiceStatusSourcePersistenceInvalidMetadataDoesNotBlockUsage(t *testing.T) {
	repo := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB)
	log := timingTestLog(t, observedTestTiming())
	log.ServiceStatusObservation = sourceTestObservation(0)
	log.ServiceStatusObservation.SchemaVersion = 999
	before := time.Now()
	inserted, err := repo.Create(context.Background(), log)
	require.NoError(t, err)
	require.True(t, inserted)
	require.True(t, service.ServiceStatusSourceErrorSince(before))
	got, err := repo.GetByID(context.Background(), log.ID)
	require.NoError(t, err)
	require.Nil(t, got.ServiceStatusObservation)
	require.Equal(t, log.InputTokens, got.InputTokens)
	require.Equal(t, log.ActualCost, got.ActualCost)
}

func TestServiceStatusSourcePersistenceBillingDedupDoesNotHideNewLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB)
	first := timingTestLog(t, observedTestTiming())
	first.ServiceStatusObservation = sourceTestObservation(0)
	inserted, err := repo.Create(ctx, first)
	require.NoError(t, err)
	require.True(t, inserted)
	duplicate := *first
	duplicate.UsageTiming = first.UsageTiming.Clone()
	duplicate.ServiceStatusObservation = sourceTestObservation(0)
	before := time.Now()
	inserted, err = repo.Create(ctx, &duplicate)
	require.NoError(t, err)
	require.False(t, inserted)
	require.True(t, service.ServiceStatusSourceErrorSince(before))
	stored, err := repo.GetByID(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, first.ServiceStatusObservation, stored.ServiceStatusObservation)
	require.Equal(t, first.ActualCost, stored.ActualCost)

	best := timingTestLog(t, observedTestTiming())
	best.ServiceStatusObservation = sourceTestObservation(0)
	require.NoError(t, repo.CreateBestEffort(ctx, best))
	best.UsageTiming = best.UsageTiming.Clone()
	best.ServiceStatusObservation = sourceTestObservation(0)
	before = time.Now()
	require.NoError(t, repo.CreateBestEffort(ctx, best)) // recent cache 同样必须检测新源键。
	require.True(t, service.ServiceStatusSourceErrorSince(before))

	// 绕过 recent cache，直接实际 best-effort ON CONFLICT 路径。
	before = time.Now()
	batch := []usageLogBestEffortRequest{{prepared: prepareUsageLogInsert(&duplicate), apiKeyID: duplicate.APIKeyID, resultCh: make(chan error, 1)}}
	repo.flushBestEffortBatch(integrationDB, batch)
	require.NoError(t, <-batch[0].resultCh)
	require.True(t, service.ServiceStatusSourceErrorSince(before))
}
