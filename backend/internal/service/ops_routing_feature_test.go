//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type routingFeatureSettingsRepo struct {
	opsRuntimeRefreshRepo
	setError error
	readGate chan struct{}
	readDone chan struct{}
	gateOnce sync.Once
}

func newRoutingFeatureSettingsRepo() *routingFeatureSettingsRepo {
	return &routingFeatureSettingsRepo{opsRuntimeRefreshRepo: opsRuntimeRefreshRepo{values: map[string]string{}}}
}

func (r *routingFeatureSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if r.fail.Load() {
		return "", errors.New("settings unavailable")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	raw, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return raw, nil
}

func (r *routingFeatureSettingsRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	values, err := r.opsRuntimeRefreshRepo.GetMultiple(ctx, keys)
	if r.readGate != nil {
		r.gateOnce.Do(func() {
			close(r.readDone)
			<-r.readGate
		})
	}
	return values, err
}

func (r *routingFeatureSettingsRepo) Set(_ context.Context, key, value string) error {
	if r.setError != nil {
		return r.setError
	}
	r.set(key, value)
	return nil
}

func TestOpsRoutingFeatureDefaultIndependentAndHotPath(t *testing.T) {
	repo := newRoutingFeatureSettingsRepo()
	repo.set(SettingKeyOpsMonitoringEnabled, "false")
	svc := &OpsService{settingRepo: repo}
	svc.initRuntimeSettings(context.Background())
	for range 1000 {
		require.True(t, svc.IsRoutingDiagnosticsEnabled(context.Background()))
		require.False(t, svc.IsMonitoringEnabled(context.Background()))
	}
	require.EqualValues(t, 1, repo.calls.Load(), "请求热路径不得读取设置表")
	cfg, err := svc.GetRoutingDiagnosticsConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, &OpsRoutingDiagnosticsConfig{Enabled: true, EffectiveEnabled: true}, cfg)
	_, err = svc.UpdateRoutingDiagnosticsConfig(context.Background(), false)
	require.NoError(t, err)
	svc.SetMonitoringEnabled(true)
	svc.storeAdvancedSettingsSnapshot(defaultOpsAdvancedSettings())
	require.False(t, svc.IsRoutingDiagnosticsEnabled(context.Background()), "其它配置发布不得覆盖诊断字段")
	repo.setError = errors.New("write failed")
	_, err = svc.UpdateRoutingDiagnosticsConfig(context.Background(), true)
	require.ErrorContains(t, err, "保存路由诊断配置失败")
	require.False(t, svc.IsRoutingDiagnosticsEnabled(context.Background()))
	raw, err := repo.GetValue(context.Background(), SettingKeyRoutingDiagnosticsEnabled)
	require.NoError(t, err)
	require.Equal(t, "false", raw)
}

func TestOpsRoutingFeatureColdFailureAndRefreshPreserveKnownState(t *testing.T) {
	repo := newRoutingFeatureSettingsRepo()
	repo.fail.Store(true)
	svc := &OpsService{settingRepo: repo}
	svc.initRuntimeSettings(context.Background())
	require.False(t, svc.IsRoutingDiagnosticsEnabled(context.Background()))
	require.True(t, svc.IsMonitoringEnabled(context.Background()), "诊断读取失败不得改变既有监控默认")
	_, err := svc.GetRoutingDiagnosticsConfig(context.Background())
	require.Error(t, err)
	repo.fail.Store(false)
	repo.set(SettingKeyRoutingDiagnosticsEnabled, "invalid")
	repo.set(SettingKeyOpsMonitoringEnabled, "false")
	repo.set(SettingKeyOpsAdvancedSettings, `{"ignore_no_available_accounts":true}`)
	require.Error(t, svc.RefreshRuntimeSettings(context.Background()))
	require.False(t, svc.IsRoutingDiagnosticsEnabled(context.Background()))
	require.False(t, svc.IsMonitoringEnabled(context.Background()))
	require.True(t, svc.OpsAdvancedSettingsSnapshot().IgnoreNoAvailableAccounts)
	_, err = svc.GetRoutingDiagnosticsConfig(context.Background())
	require.ErrorContains(t, err, "必须为 true 或 false")
	repo.set(SettingKeyRoutingDiagnosticsEnabled, "true")
	require.NoError(t, svc.RefreshRuntimeSettings(context.Background()))
	repo.fail.Store(true)
	require.Error(t, svc.RefreshRuntimeSettings(context.Background()))
	require.True(t, svc.IsRoutingDiagnosticsEnabled(context.Background()))
	repo.fail.Store(false)
	repo.set(SettingKeyRoutingDiagnosticsEnabled, "")
	require.Error(t, svc.RefreshRuntimeSettings(context.Background()))
	require.True(t, svc.IsRoutingDiagnosticsEnabled(context.Background()), "非法刷新保留已知诊断状态")
}

func TestOpsRoutingFeatureSharedRefreshAndUpdateOrder(t *testing.T) {
	repo := newRoutingFeatureSettingsRepo()
	first, second := &OpsService{settingRepo: repo}, &OpsService{settingRepo: repo}
	first.initRuntimeSettings(context.Background())
	second.initRuntimeSettings(context.Background())
	_, err := first.UpdateRoutingDiagnosticsConfig(context.Background(), false)
	require.NoError(t, err)
	cfg, err := second.GetRoutingDiagnosticsConfig(context.Background())
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.True(t, cfg.EffectiveEnabled, "跨实例配置读取应如实报告尚未刷新的本地状态")
	second.startRuntimeSettingsRefresh(context.Background(), 5*time.Millisecond, 0, time.Second)
	t.Cleanup(second.StopRuntimeSettingsRefresh)
	waitForOpsRefresh(t, time.Second, func() bool { return !second.IsRoutingDiagnosticsEnabled(context.Background()) })
	second.StopRuntimeSettingsRefresh()
	_, err = first.UpdateRoutingDiagnosticsConfig(context.Background(), true)
	require.NoError(t, err)
	repo.readGate, repo.readDone = make(chan struct{}), make(chan struct{})
	refreshResult, updateResult := make(chan error, 1), make(chan error, 1)
	go func() { refreshResult <- first.RefreshRuntimeSettings(context.Background()) }()
	<-repo.readDone
	go func() {
		_, err := first.UpdateRoutingDiagnosticsConfig(context.Background(), false)
		updateResult <- err
	}()
	close(repo.readGate)
	require.NoError(t, <-refreshResult)
	require.NoError(t, <-updateResult)
	require.False(t, first.IsRoutingDiagnosticsEnabled(context.Background()), "旧刷新不能覆盖后续成功持久化")
}

func TestOpsRoutingFeatureDisabledOwnerAndWSTurnCannotRestart(t *testing.T) {
	ctx := WithRoutingDiagnosticsEnabled(context.Background(), false)
	ctx = WithRoutingDiagnosticsRequest(ctx, 1)
	ctx = EnsureRoutingDiagnosticsRequest(ctx)
	ctx = EnsureRoutingDiagnosticsTurn(ctx, 2)
	_, b := ensureRoutingDiagnosticsSelection(ctx)
	require.Nil(t, b)
	require.Nil(t, ctx.Value(routingDiagnosticsRequestKey{}), "关闭时不创建请求归属")
	b.outcome("load_balance", "")
	b.observePool([]Account{{ID: 1}})
	b.reject(1, "excluded")
	b.pass(1)
	selection := &AccountSelectionResult{Acquired: true}
	got, diagnostic, err := finishRoutingDiagnosticsSelection(b, selection, "load_balance", ErrNoAvailableAccounts)
	require.Same(t, selection, got)
	require.Nil(t, diagnostic)
	require.Same(t, ErrNoAvailableAccounts, err)
	diagnostic, err = rejectRoutingDiagnosticsSelection(ctx, selection, s33EmptyRoutingSnapshot(), "gateway_pool_restricted", err)
	require.Nil(t, diagnostic)
	require.Same(t, ErrNoAvailableAccounts, err)
	owner := WithRoutingDiagnosticsRequest(context.Background(), 7)
	require.False(t, routingDiagnosticsEnabled(WithRoutingDiagnosticsOwner(ctx, owner)))
	bound := WithRoutingDiagnosticsOwner(context.Background(), ctx)
	require.False(t, routingDiagnosticsEnabled(bound))
	_, b = beginRoutingDiagnosticsSelection(bound)
	require.Nil(t, b)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/responses", nil).WithContext(EnsureServiceStatusRequest(ctx))
	BeginOpsStreamTurn(c, 1)
	BeginOpsStreamTurn(c, 2)
	require.Nil(t, c.Request.Context().Value(routingDiagnosticsRequestKey{}))
	require.Nil(t, GetRoutingDiagnostics(c.Request.Context()))
	require.NotNil(t, ServiceStatusAttemptObservation(c.Request.Context(), "unclassified", time.Now()))
}

func TestOpsRoutingFeaturePersistenceAndHistoryPreserveCoreError(t *testing.T) {
	ctx := context.Background()
	repo := newRoutingFeatureSettingsRepo()
	var persisted []*OpsInsertErrorLogInput
	history := &OpsErrorLogDetail{OpsErrorLog: OpsErrorLog{ID: 1, Message: "真实错误", StatusCode: 502}, ErrorBody: `{"error":"真实错误"}`, RoutingDiagnostics: s33EmptyRoutingSnapshot()}
	diagJSON, err := json.Marshal(history.RoutingDiagnostics)
	require.NoError(t, err)
	history.UpstreamErrors = `[{"message":"真实上游失败","future_field":1,"routing_diagnostics":` + string(diagJSON) + `}]`
	opsRepo := &opsRepoMock{
		InsertErrorLogFn: func(_ context.Context, entry *OpsInsertErrorLogInput) (int64, error) {
			persisted = append(persisted, entry)
			return 1, nil
		},
		BatchInsertErrorLogsFn: func(_ context.Context, entries []*OpsInsertErrorLogInput) (int64, error) {
			persisted = append(persisted, entries...)
			return int64(len(entries)), nil
		},
		GetErrorLogByIDFn: func(context.Context, int64) (*OpsErrorLogDetail, error) { return history, nil },
	}
	svc := NewOpsService(opsRepo, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	observation := &ServiceStatusObservation{SchemaVersion: 1, ObservationKey: "9aa1cbd0-b1f7-4d7c-8814-6a1d983187ec", EventRole: "terminal", TerminalKind: routingValue(CompletionStatusUpstreamError), ObservedAt: time.Now(), ReasonCode: routingValue("provider_5xx")}
	queued := &OpsInsertErrorLogInput{ErrorMessage: "真实错误", StatusCode: 502, ErrorOwner: "provider", RoutingDiagnostics: s33EmptyRoutingSnapshot(), ServiceStatusObservation: observation, UpstreamErrors: []*OpsUpstreamErrorEvent{{Message: "真实上游失败", UpstreamStatusCode: 502, RoutingDiagnostics: s33EmptyRoutingSnapshot(), ServiceStatusObservation: observation}}}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(queued))
	require.NotNil(t, queued.RoutingDiagnosticsJSON)
	queuedDiagnostic := *queued.RoutingDiagnosticsJSON
	_, err = svc.UpdateRoutingDiagnosticsConfig(ctx, false)
	require.NoError(t, err)
	require.NoError(t, svc.RecordError(ctx, queued))
	require.NoError(t, svc.RecordErrorBatch(ctx, []*OpsInsertErrorLogInput{
		{ErrorMessage: "batch one", RoutingDiagnosticsJSON: &queuedDiagnostic, UpstreamErrorsJSON: &history.UpstreamErrors},
		{ErrorMessage: "batch two", RoutingDiagnostics: s33EmptyRoutingSnapshot(), UpstreamErrors: []*OpsUpstreamErrorEvent{{Message: "batch attempt", RoutingDiagnostics: s33EmptyRoutingSnapshot()}}},
	}))
	require.Len(t, persisted, 3)
	for _, entry := range persisted {
		require.Nil(t, entry.RoutingDiagnosticsJSON)
		require.NotNil(t, entry.UpstreamErrorsJSON)
		require.NotContains(t, *entry.UpstreamErrorsJSON, "routing_diagnostics")
	}
	require.Equal(t, "真实错误", persisted[0].ErrorMessage)
	require.Equal(t, 502, persisted[0].StatusCode)
	require.Equal(t, "provider", persisted[0].ErrorOwner)
	require.Equal(t, observation, persisted[0].ServiceStatusObservation)
	require.NotNil(t, persisted[0].ServiceStatusObservationJSON)
	require.Contains(t, *persisted[0].UpstreamErrorsJSON, "真实上游失败")
	detail, err := svc.GetErrorLogByID(ctx, 1)
	require.NoError(t, err)
	require.NotSame(t, history, detail)
	require.False(t, *detail.RoutingDiagnosticsEnabled)
	require.Nil(t, detail.RoutingDiagnostics)
	require.NotContains(t, detail.UpstreamErrors, "routing_diagnostics")
	require.Contains(t, detail.UpstreamErrors, "future_field")
	require.Equal(t, history.ErrorBody, detail.ErrorBody)
	require.NotNil(t, history.RoutingDiagnostics, "裁剪不改变历史 owner")
	for _, publicDTO := range []any{ToUserErrorRequestDetail(detail), &OpsErrorLogList{Errors: []*OpsErrorLog{&detail.OpsErrorLog}}} {
		raw, marshalErr := json.Marshal(publicDTO)
		require.NoError(t, marshalErr)
		require.NotContains(t, string(raw), "routing_diagnostics_enabled", "配置元数据只属于管理单记录详情")
	}
	_, err = svc.UpdateRoutingDiagnosticsConfig(ctx, true)
	require.NoError(t, err)
	detail, err = svc.GetErrorLogByID(ctx, 1)
	require.NoError(t, err)
	require.True(t, *detail.RoutingDiagnosticsEnabled)
	require.NotNil(t, detail.RoutingDiagnostics)
	_, err = svc.UpdateRoutingDiagnosticsConfig(ctx, false)
	require.NoError(t, err)
	history.UpstreamErrors = " null "
	detail, err = svc.GetErrorLogByID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, " null ", detail.UpstreamErrors)
	history.UpstreamErrors = "invalid"
	_, err = svc.GetErrorLogByID(ctx, 1)
	require.Error(t, err, "历史格式错误必须显式报告")
}

func TestOpsRoutingFeatureDisabledTransportFallbackAndGatewayPoolReject(t *testing.T) {
	t.Run("transport_fallback", func(t *testing.T) {
		resetOpenAIAdvancedSchedulerSettingCacheForTest()
		t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
		group := int64(94210)
		accounts := newLegacySchedulerDecisionTestAccounts(group, true)
		accounts[0].Extra["openai_apikey_responses_websockets_v2_enabled"] = false
		svc := newLegacySchedulerDecisionTestService(accounts, false, schedulerTestConcurrencyCache{})
		ctx := WithRoutingDiagnosticsEnabled(context.Background(), false)
		selection, decision, err := svc.SelectAccountWithSchedulerForCapability(ctx, &group, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportResponsesWebsocketV2Ingress, OpenAIEndpointCapabilityResponses, false, false, false)
		require.NoError(t, err)
		selection.ReleaseFunc()
		require.Equal(t, accounts[1].ID, selection.Account.ID)
		require.Nil(t, selection.RoutingDiagnostics)
		require.Nil(t, decision.RoutingDiagnostics)
	})
	t.Run("gateway_pool_terminal_reject", func(t *testing.T) {
		resetOpenAIAdvancedSchedulerSettingCacheForTest()
		t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
		group := int64(94300)
		account := rotationAccount(94301, group)
		fake := newGwpoolFakePool(t, "offline", 150)
		fake.configure(account)
		released := []int64{}
		svc := routingTestService("legacy_plain", nil, schedulerTestConcurrencyCache{})
		svc.accountRepo = gatewayRotationRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{*account}}}
		svc.concurrencyService = NewConcurrencyService(routingCallbackSlots{schedulerTestConcurrencyCache: schedulerTestConcurrencyCache{releasedIDs: &released}, onAcquire: func(int64) {
			seedGatewayPoolLocalReady(svc, 0, 50)
			require.NoError(t, svc.enterGatewayPoolRest(context.Background(), account, gwpoolTestIdentity, time.Now(), time.Now().Add(time.Hour)))
		}})
		ctx := WithRoutingDiagnosticsEnabled(gatewayPoolTestGroupContext(group, 1), false)
		selection, decision, err := svc.SelectAccountWithSchedulerForImages(ctx, &group, "", "gpt-image-1", nil, OpenAIImagesCapabilityBasic)
		require.Nil(t, selection)
		require.ErrorIs(t, err, ErrNoAvailableAccounts)
		require.Nil(t, RoutingDiagnosticsFromError(err))
		require.Nil(t, decision.RoutingDiagnostics)
		require.Equal(t, []int64{account.ID}, released)
	})
}

func TestOpsRoutingFeatureDisabledRealSelectionPreservesResultAndSlots(t *testing.T) {
	for _, mode := range []string{"advanced", "legacy_batch", "legacy_plain"} {
		t.Run(mode, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
			for _, outcome := range []string{"empty", "acquired", "wait", "compact"} {
				t.Run(outcome, func(t *testing.T) {
					account := routingDiagnosticsTestAccount(94001)
					accounts := []Account{account}
					released := []int64{}
					slots := schedulerTestConcurrencyCache{releasedIDs: &released}
					if outcome == "empty" {
						accounts = nil
					}
					if outcome == "wait" {
						slots.acquireResults = map[int64]bool{account.ID: false}
					}
					if outcome == "compact" {
						accounts[0].Extra = map[string]any{"openai_compact_supported": false}
					}
					svc := routingTestService(mode, accounts, slots)
					ctx := WithRoutingDiagnosticsEnabled(context.Background(), false)
					selection, decision, err := routingSelect(svc, ctx, "gpt-5.1", nil, outcome == "compact")
					require.Nil(t, decision.RoutingDiagnostics)
					require.Nil(t, RoutingDiagnosticsFromError(err))
					switch outcome {
					case "empty", "compact":
						require.Nil(t, selection)
						if outcome == "empty" {
							require.ErrorIs(t, err, ErrNoAvailableAccounts)
						} else {
							require.ErrorIs(t, err, ErrNoAvailableCompactAccounts)
						}
					case "acquired":
						require.NoError(t, err)
						require.Equal(t, account.ID, selection.Account.ID)
						require.True(t, selection.Acquired)
						require.Nil(t, selection.RoutingDiagnostics)
						selection.ReleaseFunc()
						require.Equal(t, []int64{account.ID}, released)
					case "wait":
						require.NoError(t, err)
						require.NotNil(t, selection.WaitPlan)
						require.False(t, selection.Acquired)
						require.Nil(t, selection.RoutingDiagnostics)
					}
				})
			}
		})
	}
}
