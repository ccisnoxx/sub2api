//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func routingTestAccount(id int64) Account {
	return Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}
}

func routingTestService(mode string, accounts []Account, slots schedulerTestConcurrencyCache) *OpenAIGatewayService {
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = mode == "legacy_batch"
	cfg.Gateway.OpenAIWS.LBTopK = 1
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: &schedulerTestGatewayCache{}, cfg: cfg, concurrencyService: NewConcurrencyService(slots)}
	if mode == "advanced" {
		svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
	}
	return svc
}

func routingSelect(svc *OpenAIGatewayService, ctx context.Context, model string, excluded map[int64]struct{}, compact bool) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return svc.SelectAccountWithScheduler(ctx, nil, "", "", model, excluded, OpenAIUpstreamTransportAny, compact)
}

func requireRoutingCounts(t *testing.T, d *RoutingDiagnostics, pool, filtered int, coverage, reason string, reasons map[string]int) {
	t.Helper()
	require.NotNil(t, d)
	require.Equal(t, pool, *d.CandidatePool)
	require.Equal(t, filtered, *d.FilteredCandidates)
	require.Equal(t, coverage, d.FilterCoverage)
	require.Equal(t, reason, *d.SelectionReason)
	require.Equal(t, reasons, d.FilterReasons)
}

func TestOpenAIRoutingDiagnosticsAdvancedAndLegacyResults(t *testing.T) {
	for _, mode := range []string{"advanced", "legacy_batch", "legacy_plain"} {
		t.Run(mode, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
			t.Run("empty_pool", func(t *testing.T) {
				svc := routingTestService(mode, nil, schedulerTestConcurrencyCache{})
				ctx := WithRoutingDiagnosticsRequest(context.Background(), 0)
				result, decision, err := routingSelect(svc, ctx, "gpt-5.1", nil, false)
				require.Nil(t, result)
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
				requireRoutingCounts(t, RoutingDiagnosticsFromError(err), 0, 0, "complete", "pool_empty", map[string]int{})
				require.Equal(t, decision.RoutingDiagnostics, GetRoutingDiagnostics(ctx))
			})
			t.Run("duplicate_ids_first_exclusion", func(t *testing.T) {
				bad, good := routingTestAccount(92001), routingTestAccount(92002)
				bad.Schedulable = false
				svc := routingTestService(mode, []Account{bad, bad, good}, schedulerTestConcurrencyCache{})
				selection, decision, err := routingSelect(svc, context.Background(), "gpt-5.1", map[int64]struct{}{bad.ID: {}}, false)
				require.NoError(t, err)
				t.Cleanup(selection.ReleaseFunc)
				require.Equal(t, good.ID, selection.Account.ID)
				requireRoutingCounts(t, selection.RoutingDiagnostics, 2, 1, "complete", "slot_acquired", map[string]int{"excluded": 1})
				require.Equal(t, decision.RoutingDiagnostics, selection.RoutingDiagnostics)
			})
			t.Run("duplicate_all_filtered", func(t *testing.T) {
				account := routingTestAccount(92003)
				svc := routingTestService(mode, []Account{account, account}, schedulerTestConcurrencyCache{})
				_, _, err := routingSelect(svc, context.Background(), "gpt-5.1", map[int64]struct{}{account.ID: {}}, false)
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
				requireRoutingCounts(t, RoutingDiagnosticsFromError(err), 1, 1, "complete", "candidates_filtered", map[string]int{"excluded": 1})
			})
			t.Run("all_model_rejections", func(t *testing.T) {
				accounts := []Account{routingTestAccount(92011), routingTestAccount(92012)}
				for i := range accounts {
					accounts[i].Credentials = map[string]any{"model_mapping": map[string]any{"only-model": "only-model"}}
				}
				svc := routingTestService(mode, accounts, schedulerTestConcurrencyCache{})
				_, _, err := routingSelect(svc, context.Background(), "other-model", nil, false)
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
				requireRoutingCounts(t, RoutingDiagnosticsFromError(err), 2, 2, "complete", "candidates_filtered", map[string]int{"model_not_supported": 2})
			})
			t.Run("wait_is_not_capacity_exclusion", func(t *testing.T) {
				account := routingTestAccount(92021)
				svc := routingTestService(mode, []Account{account}, schedulerTestConcurrencyCache{acquireResults: map[int64]bool{account.ID: false}})
				selection, _, err := routingSelect(svc, context.Background(), "gpt-5.1", nil, false)
				require.NoError(t, err)
				require.NotNil(t, selection.WaitPlan)
				require.False(t, selection.Acquired)
				requireRoutingCounts(t, selection.RoutingDiagnostics, 1, 0, "complete", "wait_plan", map[string]int{})
			})
			t.Run("compact_final_rejection", func(t *testing.T) {
				account := routingTestAccount(92031)
				account.Extra = map[string]any{"openai_compact_supported": false}
				svc := routingTestService(mode, []Account{account}, schedulerTestConcurrencyCache{})
				_, _, err := routingSelect(svc, context.Background(), "gpt-5.1", nil, true)
				require.ErrorIs(t, err, ErrNoAvailableCompactAccounts)
				requireRoutingCounts(t, RoutingDiagnosticsFromError(err), 1, 1, "complete", "compact_unsupported", map[string]int{"compact_unsupported": 1})
			})
		})
	}
}

type routingListErrorRepo struct {
	schedulerTestOpenAIAccountRepo
	failure error
}

func (r routingListErrorRepo) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	return nil, r.failure
}

func TestOpenAIRoutingDiagnosticsListFailureAndChannelClearPriorPool(t *testing.T) {
	for _, mode := range []string{"advanced", "legacy_batch", "legacy_plain"} {
		t.Run(mode, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			ctx := WithRoutingDiagnosticsRequest(context.Background(), 0)
			svc := routingTestService(mode, []Account{routingTestAccount(92101)}, schedulerTestConcurrencyCache{})
			selection, _, err := routingSelect(svc, ctx, "gpt-5.1", nil, false)
			require.NoError(t, err)
			selection.ReleaseFunc()
			old := selection.RoutingDiagnostics.Clone()
			cause := errors.New("list failed")
			svc.accountRepo = routingListErrorRepo{failure: cause}
			_, decision, err := routingSelect(svc, ctx, "gpt-5.1", nil, false)
			require.ErrorIs(t, err, cause)
			d := RoutingDiagnosticsFromError(fmt.Errorf("外层: %w", err))
			require.Equal(t, 2, d.SelectionAttempt)
			require.Equal(t, "account_list_failed", *d.SelectionReason)
			require.Nil(t, d.CandidatePool)
			require.Nil(t, d.FilterReasons)
			require.Equal(t, "unobserved", d.FilterCoverage)
			require.Equal(t, d, decision.RoutingDiagnostics)
			require.Equal(t, old, selection.RoutingDiagnostics)

			group := int64(92110)
			svc.channelService = newTestChannelService(makeStandardRepo(Channel{ID: 92111, Status: StatusActive, GroupIDs: []int64{group}, RestrictModels: true, BillingModelSource: BillingModelSourceChannelMapped, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"gpt-4o"}}}, ModelMapping: map[string]map[string]string{PlatformOpenAI: {"gpt-5.1": "o3-mini"}}}, map[int64]string{group: PlatformOpenAI}))
			// 强 previous-response 绑定跳过 gwpool preference 列表，验证真实预检查先拒绝。
			_, decision, err = svc.SelectAccountWithScheduler(ctx, &group, "resp-pricing", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			d = RoutingDiagnosticsFromError(err)
			require.Equal(t, 3, d.SelectionAttempt)
			require.Equal(t, "channel_pricing", *d.SelectionLayer)
			require.Equal(t, "channel_pricing_restricted", *d.SelectionReason)
			require.Nil(t, d.CandidatePool)
			require.Nil(t, d.FilteredCandidates)
			require.Equal(t, d, decision.RoutingDiagnostics)
		})
	}
}

func TestOpenAIRoutingDiagnosticsStickyAndLegacyTransportReevaluation(t *testing.T) {
	for _, mode := range []string{"advanced", "legacy_batch", "legacy_plain"} {
		t.Run(mode, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			account := routingTestAccount(92201)
			svc := routingTestService(mode, []Account{account}, schedulerTestConcurrencyCache{})
			ctx := WithRoutingDiagnosticsRequest(context.Background(), 0)
			require.NoError(t, svc.setStickySessionAccountID(ctx, nil, "sticky", account.ID, time.Hour))
			selection, decision, err := svc.SelectAccountWithScheduler(ctx, nil, "", "sticky", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			selection.ReleaseFunc()
			require.Equal(t, "session_hash", *selection.RoutingDiagnostics.SelectionLayer)
			require.Equal(t, "sticky_hit", *selection.RoutingDiagnostics.SelectionReason)
			require.Equal(t, selection.RoutingDiagnostics, decision.RoutingDiagnostics)
			if mode != "legacy_batch" {
				require.Nil(t, selection.RoutingDiagnostics.CandidatePool)
				require.Equal(t, "unobserved", selection.RoutingDiagnostics.FilterCoverage)
			} else {
				require.Equal(t, 1, *selection.RoutingDiagnostics.CandidatePool)
				require.Equal(t, "partial", selection.RoutingDiagnostics.FilterCoverage)
			}
		})
	}
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	group := int64(92210)
	accounts := newLegacySchedulerDecisionTestAccounts(group, true)
	accounts[0].Extra["openai_apikey_responses_websockets_v2_enabled"] = false
	svc := newLegacySchedulerDecisionTestService(accounts, false, schedulerTestConcurrencyCache{})
	ctx := WithRoutingDiagnosticsRequest(context.Background(), 0)
	selection, _, err := svc.SelectAccountWithSchedulerForCapability(ctx, &group, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportResponsesWebsocketV2Ingress, OpenAIEndpointCapabilityResponses, false, false, false)
	require.NoError(t, err)
	selection.ReleaseFunc()
	require.Equal(t, accounts[1].ID, selection.Account.ID)
	require.Equal(t, 2, selection.RoutingDiagnostics.SelectionAttempt)
	requireRoutingCounts(t, selection.RoutingDiagnostics, 2, 1, "complete", "slot_acquired", map[string]int{"excluded": 1})
}

func TestOpenAIRoutingDiagnosticsAdvancedPartialTopKAndCompactRecovery(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	svc := routingTestService("advanced", []Account{routingTestAccount(92301), routingTestAccount(92302)}, schedulerTestConcurrencyCache{})
	selection, decision, err := routingSelect(svc, context.Background(), "gpt-5.1", nil, false)
	require.NoError(t, err)
	selection.ReleaseFunc()
	require.Equal(t, 1, decision.TopK)
	requireRoutingCounts(t, selection.RoutingDiagnostics, 2, 0, "partial", "slot_acquired", map[string]int{})

	for _, mode := range []string{"advanced", "legacy_batch", "legacy_plain"} {
		t.Run(mode, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			stale := routingTestAccount(92311)
			stale.Extra = map[string]any{"openai_compact_supported": false}
			fresh := stale
			fresh.Extra = map[string]any{"openai_compact_supported": true}
			svc := routingTestService(mode, []Account{fresh}, schedulerTestConcurrencyCache{})
			svc.schedulerSnapshot = &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{snapshotAccounts: []*Account{&stale}, accountsByID: map[int64]*Account{fresh.ID: &fresh}}}
			selection, _, err := routingSelect(svc, context.Background(), "gpt-5.1", nil, true)
			require.NoError(t, err)
			selection.ReleaseFunc()
			requireRoutingCounts(t, selection.RoutingDiagnostics, 1, 0, "complete", "slot_acquired", map[string]int{})
		})
	}
}

func TestOpenAIRoutingDiagnosticsGrokObservedEntranceAndInPlaceFilter(t *testing.T) {
	for _, filter := range []string{"free", "team_all", "team_partial", "model"} {
		t.Run(filter, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			a, b := routingTestAccount(92401), routingTestAccount(92402)
			a.Platform, b.Platform = PlatformGrok, PlatformGrok
			a.Type, b.Type = AccountTypeOAuth, AccountTypeOAuth
			a.Credentials = map[string]any{"team_id": "diagnostics-" + t.Name()}
			b.Credentials = map[string]any{"team_id": "diagnostics-" + t.Name()}
			accounts := []Account{a, b}
			svc := routingTestService("advanced", accounts, schedulerTestConcurrencyCache{})
			reason, wantFiltered := "grok_team_model_rate_limit", 2
			switch filter {
			case "free":
				reason = "grok_free_quota_soft_gate"
				svc.cfg = grokFreeQuotaTestConfig()
				svc.usageLogRepo = &grokFreeQuotaUsageRepoStub{}
				for i := range accounts {
					accounts[i].Type = AccountTypeOAuth
					accounts[i].Credentials["subscription_tier"] = "free"
					openaiGrokFreeQuotaGateCache.Store(accounts[i].ID, grokFreeQuotaGateCacheEntry{tokens: 500_000, checkedAt: time.Now(), known: true})
					t.Cleanup(func() { openaiGrokFreeQuotaGateCache.Delete(accounts[i].ID) })
				}
				svc.accountRepo = schedulerTestOpenAIAccountRepo{accounts: accounts}
			case "team_all":
				markGrokTeamModelRateLimit(&a, "grok-4.5", time.Now().Add(time.Hour))
			case "team_partial":
				accounts[1].Credentials["team_id"] = "diagnostics-healthy-" + t.Name()
				markGrokTeamModelRateLimit(&a, "grok-4.5", time.Now().Add(time.Hour))
				wantFiltered = 1
			case "model":
				reason = "grok_model_quota_block"
				for i := range accounts {
					markGrokModelQuotaBlock(accounts[i].ID, "grok-4.5", time.Now().Add(time.Hour))
				}
			}
			selection, _, err := svc.SelectAccountWithSchedulerForCapability(context.Background(), nil, "", "", "grok-4.5", nil, OpenAIUpstreamTransportAny, "", false, false, false, PlatformGrok)
			d := RoutingDiagnosticsFromError(err)
			selectionReason := "candidates_filtered"
			if filter == "team_partial" {
				require.NoError(t, err)
				selection.ReleaseFunc()
				require.Equal(t, b.ID, selection.Account.ID)
				d, selectionReason = selection.RoutingDiagnostics, "slot_acquired"
			} else {
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
			}
			requireRoutingCounts(t, d, 2, wantFiltered, "complete", selectionReason, map[string]int{reason: wantFiltered})
		})
	}
}

func TestOpenAIRoutingDiagnosticsSchedulingThresholdPreservesOriginalPool(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	accountSchedulingThresholdsSF.Forget(SettingKeyAccountSchedulingThresholds)
	accountSchedulingThresholdsCache.Store(&cachedAccountSchedulingThresholds{})
	t.Cleanup(func() { accountSchedulingThresholdsCache.Store(&cachedAccountSchedulingThresholds{}) })
	settingsRepo := newMockSettingRepo()
	settingsRepo.data[SettingKeyAccountSchedulingThresholds] = `{"openai":80}`
	pauseRepo := &rateLimitAccountRepoStub{}
	rl := NewRateLimitService(pauseRepo, nil, &config.Config{}, nil, nil)
	rl.SetSettingService(NewSettingService(settingsRepo, &config.Config{}))
	account := routingTestAccount(92501)
	account.Extra = map[string]any{"codex_7d_used_percent": 91.5, "codex_7d_reset_at": time.Now().Add(time.Hour).Format(time.RFC3339)}
	svc := routingTestService("legacy_plain", []Account{account}, schedulerTestConcurrencyCache{})
	svc.rateLimitService = rl
	_, _, err := routingSelect(svc, context.Background(), "gpt-5.1", nil, false)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	requireRoutingCounts(t, RoutingDiagnosticsFromError(err), 1, 1, "complete", "candidates_filtered", map[string]int{"scheduling_threshold": 1})
	require.Equal(t, 1, pauseRepo.tempCalls, "诊断不能重跑有副作用的阈值检查")
}

type routingChangingListRepo struct {
	schedulerTestOpenAIAccountRepo
	calls int
}

func (r *routingChangingListRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	r.calls++
	if r.calls == 1 {
		return nil, nil
	}
	return r.schedulerTestOpenAIAccountRepo.ListSchedulableUngroupedByPlatform(ctx, platform)
}

func TestOpenAIRoutingDiagnosticsImagesFallbackAndProxySecondRound(t *testing.T) {
	for _, mode := range []string{"advanced", "legacy_plain"} {
		t.Run(mode+"_images", func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			svc := routingTestService(mode, nil, schedulerTestConcurrencyCache{})
			repo := &routingChangingListRepo{schedulerTestOpenAIAccountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{routingTestAccount(92601)}}}
			svc.accountRepo = repo
			selection, _, err := svc.SelectAccountWithSchedulerForImages(context.Background(), nil, "", "gpt-image-1", nil, OpenAIImagesCapabilityNative)
			require.NoError(t, err)
			selection.ReleaseFunc()
			require.Equal(t, 2, repo.calls)
			require.Equal(t, 2, selection.RoutingDiagnostics.SelectionAttempt)
			requireRoutingCounts(t, selection.RoutingDiagnostics, 1, 0, "complete", "slot_acquired", map[string]int{})
		})
		t.Run(mode+"_proxy", func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			proxy := int64(92620)
			account := routingTestAccount(92621)
			account.Type, account.ProxyID = AccountTypeOAuth, &proxy
			svc := routingTestService(mode, []Account{account}, schedulerTestConcurrencyCache{})
			svc.openaiProxyStreamCircuit = newOpenAIProxyStreamCircuit(openAIProxyStreamCircuitSettings{failureThreshold: 1, failureWindow: time.Minute, quarantineTTL: time.Hour, maxEntries: 16})
			svc.openaiProxyStreamCircuit.recordFailure(proxy, time.Now())
			selection, _, err := routingSelect(svc, context.Background(), "gpt-5.1", nil, false)
			require.NoError(t, err)
			selection.ReleaseFunc()
			require.Equal(t, 2, selection.RoutingDiagnostics.SelectionAttempt)
			requireRoutingCounts(t, selection.RoutingDiagnostics, 1, 0, "complete", "slot_acquired", map[string]int{})
			require.True(t, svc.openaiProxyStreamCircuit.isBlocked(proxy, time.Now()))
		})
	}
}

type routingCallbackSlots struct {
	schedulerTestConcurrencyCache
	onAcquire func(int64)
}

func (c routingCallbackSlots) AcquireAccountSlot(ctx context.Context, id int64, maxConcurrency int, requestID string) (bool, error) {
	c.onAcquire(id)
	return c.schedulerTestConcurrencyCache.AcquireAccountSlot(ctx, id, maxConcurrency, requestID)
}

func TestOpenAIRoutingDiagnosticsGatewayPoolReevaluationAndDeduplication(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	group := int64(92700)
	first := preferenceAccount(92701, group, 20)
	first.Priority = -1
	second := routingTestAccount(92702)
	second.GroupIDs = []int64{group}
	fake := newGwpoolFakePool(t, "offline", 150)
	fake.listGateways = []gwpoolFakeGateway{{Name: "unified-300", PairReady: true}}
	fake.configure(first)
	released := []int64{}
	svc := routingTestService("legacy_plain", nil, schedulerTestConcurrencyCache{})
	svc.accountRepo = gatewayRotationRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{*first, second}}}
	identity := openAIGatewayPoolAccountKey(first)
	svc.codexCookies.poolPairs.Store(identity, openAIGatewayPoolPair{cookie: "offline", gateway: "unified-200", version: "full", until: time.Now().Add(time.Minute)})
	svc.codexCookies.gatewayPoolMarkVerifiedFull(identity, "full")
	svc.concurrencyService = NewConcurrencyService(routingCallbackSlots{schedulerTestConcurrencyCache: schedulerTestConcurrencyCache{releasedIDs: &released}, onAcquire: func(id int64) {
		if id == first.ID {
			svc.codexCookies.poolRounds.rest(group, identity, time.Now().Add(time.Hour))
		}
	}})
	ctx := WithRoutingDiagnosticsRequest(gatewayPoolTestGroupContext(group, 2), 0)
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &group, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	selection.ReleaseFunc()
	require.Equal(t, second.ID, selection.Account.ID)
	require.Equal(t, 2, selection.RoutingDiagnostics.SelectionAttempt)
	require.Contains(t, released, first.ID)
	requireRoutingCounts(t, selection.RoutingDiagnostics, 2, 1, "complete", "slot_acquired", map[string]int{"excluded": 1})

	// 同凭证域账号在真实候选去重 owner 被移除，只计账号行，不暴露域。
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	accounts := []Account{routingTestAccount(92711), routingTestAccount(92712)}
	svc = routingTestService("advanced", accounts, schedulerTestConcurrencyCache{})
	ctx = context.WithValue(context.Background(), gatewayPoolCandidateDomainsKey{}, map[int64]string{accounts[0].ID: "private-domain", accounts[1].ID: "private-domain"})
	selection, _, err = routingSelect(svc, ctx, "gpt-5.1", nil, false)
	require.NoError(t, err)
	selection.ReleaseFunc()
	requireRoutingCounts(t, selection.RoutingDiagnostics, 2, 1, "complete", "slot_acquired", map[string]int{"gateway_pool_duplicate": 1})
}

func TestOpenAIRoutingDiagnosticsTokenCountRemainsUnobserved(t *testing.T) {
	svc := routingTestService("legacy_plain", []Account{routingTestAccount(92801)}, schedulerTestConcurrencyCache{})
	ctx := WithRoutingDiagnosticsRequest(context.Background(), 0)
	account, err := svc.SelectAccountForTokenCount(ctx, nil, "", "gpt-5.1", "", PlatformOpenAI)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Nil(t, GetRoutingDiagnostics(ctx))
}

func TestOpenAIRoutingDiagnosticsAdvancedStickyOwnersAndSubscriptionPool(t *testing.T) {
	for _, owner := range []string{"previous_response_id", "guardian_parent", "weighted_fallback", "subscription"} {
		t.Run(owner, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			group := int64(92900)
			first, second := routingTestAccount(92901), routingTestAccount(92902)
			first.GroupIDs, second.GroupIDs = []int64{group}, []int64{group}
			cfg := newSchedulerTestOpenAIWSV2Config()
			first.Extra = map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}
			slots := schedulerTestConcurrencyCache{}
			if owner == "weighted_fallback" {
				slots.acquireResults = map[int64]bool{first.ID: false, second.ID: false}
			}
			if owner == "subscription" {
				first.Type = AccountTypeOAuth
				first.Credentials = map[string]any{"plan_type": "team"}
			}
			svc := routingTestService("advanced", []Account{first, second}, slots)
			svc.cfg = cfg
			scheduler := &defaultOpenAIAccountScheduler{service: svc, stats: newOpenAIAccountRuntimeStats()}
			ctx := WithRoutingDiagnosticsRequest(context.Background(), 3)
			req := OpenAIAccountScheduleRequest{GroupID: &group, Platform: PlatformOpenAI, RequestedModel: "gpt-5.1", SessionHash: "owner-session"}
			wantLayer, wantReason := owner, "sticky_hit"
			switch owner {
			case "previous_response_id":
				req.PreviousResponseID = "resp-diagnostics-owner"
				require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(ctx, group, req.PreviousResponseID, first.ID, time.Hour))
			case "guardian_parent":
				req.GuardianParentAccountID = first.ID
			case "weighted_fallback":
				req.StickyWeighted, req.StickyAccountID = true, first.ID
				wantLayer, wantReason = "session_hash", "wait_plan"
			case "subscription":
				req.SubscriptionPriority = true
				wantLayer, wantReason = "load_balance", "slot_acquired"
			}
			selection, decision, err := scheduler.Select(ctx, req)
			require.NoError(t, err)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			d := selection.RoutingDiagnostics
			require.Equal(t, wantLayer, *d.SelectionLayer)
			require.Equal(t, wantReason, *d.SelectionReason)
			require.Equal(t, 3, *d.Turn)
			require.Equal(t, 1, d.SelectionAttempt)
			if owner == "weighted_fallback" || owner == "subscription" {
				require.Equal(t, 2, *d.CandidatePool)
				require.Equal(t, "partial", d.FilterCoverage)
				require.Zero(t, *d.FilteredCandidates)
			} else {
				require.Nil(t, d.CandidatePool)
			}
			if owner == "subscription" {
				require.Equal(t, 1, decision.CandidateCount, "旧指标继续描述订阅子池；新诊断保存完整入口")
			}
			if owner == "weighted_fallback" {
				require.Equal(t, "load_balance", decision.Layer, "真实 fallback 诊断不得改变旧指标层")
			}
		})
	}
}

func TestOpenAIRoutingDiagnosticsRepeatedDBRecheckExcludesOnce(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	stale := routingTestAccount(93001)
	fresh := stale
	fresh.GroupIDs = []int64{93002} // DB 行已经移到其他分组，同一选择的重复复核均拒绝。
	svc := routingTestService("advanced", []Account{fresh}, schedulerTestConcurrencyCache{})
	svc.schedulerSnapshot = &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{snapshotAccounts: []*Account{&stale}, accountsByID: map[int64]*Account{stale.ID: &stale}}}
	_, _, err := routingSelect(svc, context.Background(), "gpt-5.1", nil, false)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	requireRoutingCounts(t, RoutingDiagnosticsFromError(err), 1, 1, "complete", "candidates_filtered", map[string]int{"recheck_rejected": 1})
}

func TestOpenAIRoutingDiagnosticsGatewayPoolTerminalRestriction(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	group := int64(93100)
	account := rotationAccount(93101, group)
	fake := newGwpoolFakePool(t, "offline", 150)
	fake.configure(account)
	released := []int64{}
	svc := routingTestService("legacy_plain", nil, schedulerTestConcurrencyCache{})
	svc.accountRepo = gatewayRotationRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{*account}}}
	svc.concurrencyService = NewConcurrencyService(routingCallbackSlots{schedulerTestConcurrencyCache: schedulerTestConcurrencyCache{releasedIDs: &released}, onAcquire: func(int64) {
		// 选号检查后、终检前发生真实休息状态转换，必须释放已取槽位并保持同轮入口事实。
		seedGatewayPoolLocalReady(svc, 0, 50)
		require.NoError(t, svc.enterGatewayPoolRest(context.Background(), account, gwpoolTestIdentity, time.Now(), time.Now().Add(time.Hour)))
	}})
	ctx := WithRoutingDiagnosticsRequest(gatewayPoolTestGroupContext(group, 1), 0)
	selection, decision, err := svc.SelectAccountWithSchedulerForImages(ctx, &group, "", "gpt-image-1", nil, OpenAIImagesCapabilityBasic)
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	d := RoutingDiagnosticsFromError(err)
	require.Equal(t, 1, d.SelectionAttempt)
	require.Equal(t, "gateway_pool", *d.SelectionLayer)
	requireRoutingCounts(t, d, 1, 0, "partial", "gateway_pool_restricted", map[string]int{})
	require.Equal(t, []int64{account.ID}, released)
	require.Equal(t, d, decision.RoutingDiagnostics)
	require.Equal(t, d, GetRoutingDiagnostics(ctx))
}
