package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func responsesStickySpilloverAccounts(groupID int64) []Account {
	return []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
			Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
			Extra: map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
			Schedulable: true, Concurrency: 1, Priority: 1, GroupIDs: []int64{groupID},
			Extra: map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}},
	}
}

func responsesStickySpilloverService(accounts []Account, loadBatch bool, concurrency schedulerTestConcurrencyCache) *OpenAIGatewayService {
	cfg := newSchedulerTestOpenAIWSV2Config()
	cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 2 * time.Second
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
	cfg.Gateway.Scheduling.FallbackWaitTimeout = time.Second
	cfg.Gateway.Scheduling.FallbackMaxWaiting = 5
	return &OpenAIGatewayService{
		accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}},
		cache:       &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:spillover": 1}},
		cfg:         cfg, concurrencyService: NewConcurrencyService(concurrency),
	}
}

func TestResponsesStickySpillover_LegacyTriesCapacityBeforeWait(t *testing.T) {
	groupID := int64(48201)
	for _, loadBatch := range []bool{false, true} {
		for _, state := range []string{"available", "all_busy", "escape_disabled", "queue_full", "load_batch_error"} {
			t.Run(fmt.Sprintf("load_batch_%t/%s", loadBatch, state), func(t *testing.T) {
				var acquired, released []int64
				concurrency := schedulerTestConcurrencyCache{
					acquireResults: map[int64]bool{1: false, 2: state != "all_busy"},
					acquiredIDs:    &acquired, releasedIDs: &released,
					loadMap: map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 100}, 2: {AccountID: 2}},
				}
				if state == "queue_full" {
					concurrency.waitCounts = map[int64]int{1: 3}
				}
				if state == "load_batch_error" {
					concurrency.loadBatchErr = errors.New("负载批量读取失败")
				}
				svc := responsesStickySpilloverService(responsesStickySpilloverAccounts(groupID), loadBatch, concurrency)
				if state == "escape_disabled" {
					svc.cfg.Gateway.OpenAIScheduler.StickyEscapeTTFTMs = 15000
					svc.cfg.Gateway.OpenAIScheduler.StickyEscapeErrorRate = 0.5
				}

				selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "spillover", "gpt-5.1", nil)
				require.NoError(t, err)
				require.NotNil(t, selection)
				require.Equal(t, state != "escape_disabled", selection.PreserveStickyBinding)
				if state == "all_busy" || state == "escape_disabled" {
					require.Equal(t, int64(1), selection.Account.ID)
					require.False(t, selection.Acquired)
					require.Nil(t, selection.ReleaseFunc)
					require.NotNil(t, selection.WaitPlan)
					require.Equal(t, 2*time.Second, selection.WaitPlan.Timeout)
					require.Equal(t, 3, selection.WaitPlan.MaxWaiting)
					require.True(t, selection.stickySessionHit)
					if state == "escape_disabled" {
						require.NotContains(t, acquired, int64(2))
					} else {
						require.Contains(t, acquired, int64(2), "无可用槽位前必须尝试兼容账号")
					}
				} else {
					require.Equal(t, int64(2), selection.Account.ID)
					require.True(t, selection.Acquired)
					require.Nil(t, selection.WaitPlan)
					require.False(t, selection.stickySessionHit)
					selection.ReleaseFunc()
					require.Equal(t, []int64{2}, released)
				}
				cache, ok := svc.cache.(*schedulerTestGatewayCache)
				require.True(t, ok)
				require.Equal(t, int64(1), cache.sessionBindings["openai:spillover"], "临时满载分流不能改写原绑定")
			})
		}
	}
}

func TestResponsesStickySpillover_LegacyPreservesAdmissionGates(t *testing.T) {
	// 同进程的其他调度测试会缓存高级开关；本用例必须固定走 legacy 路径。
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	groupID := int64(48202)
	for _, loadBatch := range []bool{false, true} {
		for _, gate := range []string{"model", "group", "capability", "profit", "excluded", "transport"} {
			t.Run(fmt.Sprintf("load_batch_%t/%s", loadBatch, gate), func(t *testing.T) {
				accounts := responsesStickySpilloverAccounts(groupID)
				var acquired, released []int64
				concurrency := schedulerTestConcurrencyCache{
					acquireResults: map[int64]bool{1: false, 2: true},
					acquiredIDs:    &acquired, releasedIDs: &released,
					loadMap: map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 100}, 2: {AccountID: 2}},
				}
				ctx := context.Background()
				var excluded map[int64]struct{}
				transport := OpenAIUpstreamTransportAny
				switch gate {
				case "model":
					accounts[1].Credentials = map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}
				case "group":
					accounts[1].GroupIDs = []int64{groupID + 1}
				case "capability":
					accounts[1].Extra["openai_responses_supported"] = false
				case "profit":
					profitControlTestAccountWithRate(&accounts[0], 0.3)
					profitControlTestAccountWithRate(&accounts[1], 0.8)
					ctx = profitControlTestCtx(profitControlTestGroup(groupID, 0.5, 0))
				case "excluded":
					excluded = map[int64]struct{}{2: {}}
				case "transport":
					transport = OpenAIUpstreamTransportResponsesWebsocketV2Ingress
					accounts[1].Extra["openai_apikey_responses_websockets_v2_enabled"] = false
				}
				svc := responsesStickySpilloverService(accounts, loadBatch, concurrency)
				selection, decision, err := svc.SelectAccountWithSchedulerForCapability(
					ctx, &groupID, "", "spillover", "gpt-5.1", excluded, transport,
					OpenAIEndpointCapabilityResponses, false, false, true,
				)
				require.NoError(t, err)
				require.NotNil(t, selection)
				require.Equal(t, int64(1), selection.Account.ID)
				require.False(t, selection.Acquired)
				require.NotNil(t, selection.WaitPlan)
				require.True(t, decision.StickySessionHit)
				require.True(t, selection.PreserveStickyBinding)
				if gate == "transport" {
					// legacy 调度后的 transport 终检拒绝候选时，必须释放已经抢到的槽位。
					require.Contains(t, acquired, int64(2))
					require.Equal(t, []int64{2}, released)
				} else {
					require.NotContains(t, acquired, int64(2))
					require.Empty(t, released)
				}
				cache, ok := svc.cache.(*schedulerTestGatewayCache)
				require.True(t, ok)
				require.Equal(t, int64(1), cache.sessionBindings["openai:spillover"])
			})
		}
	}
}

func TestResponsesStickySpillover_WeightedFallbackTriesOtherStickyBeforeWait(t *testing.T) {
	groupID := int64(48203)
	for _, state := range []string{"available", "all_busy", "escape_disabled", "owner_bound", "incompatible"} {
		t.Run(state, func(t *testing.T) {
			accounts := responsesStickySpilloverAccounts(groupID)
			if state == "incompatible" {
				accounts[1].Extra["openai_responses_supported"] = false
			}
			var acquired, released []int64
			svc := responsesStickySpilloverService(accounts, true, schedulerTestConcurrencyCache{
				acquireResults: map[int64]bool{1: false, 2: state != "all_busy"},
				acquiredIDs:    &acquired, releasedIDs: &released,
			})
			if state == "escape_disabled" {
				svc.cfg.Gateway.OpenAIScheduler.StickyEscapeTTFTMs = 15000
				svc.cfg.Gateway.OpenAIScheduler.StickyEscapeErrorRate = 0.5
			}
			scheduler := &defaultOpenAIAccountScheduler{service: svc, stats: newOpenAIAccountRuntimeStats()}
			selection, err := scheduler.tryFallbackToWeightedSticky(context.Background(), OpenAIAccountScheduleRequest{
				GroupID: &groupID, Platform: PlatformOpenAI, SessionHash: "spillover",
				StickyWeighted: true, StickyPreviousAccountID: 1, StickyAccountID: 2,
				PreviousResponseCanMove: true, DisableStickyEscape: state == "owner_bound",
				RequestedModel: "gpt-5.1", RequiredCapability: OpenAIEndpointCapabilityResponses,
			})
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, state != "escape_disabled" && state != "owner_bound", selection.PreserveStickyBinding)
			if state == "available" {
				require.Equal(t, int64(2), selection.Account.ID)
				require.True(t, selection.Acquired)
				require.Nil(t, selection.WaitPlan)
				selection.ReleaseFunc()
				require.Equal(t, []int64{2}, released)
			} else {
				require.Equal(t, int64(1), selection.Account.ID)
				require.False(t, selection.Acquired)
				require.NotNil(t, selection.WaitPlan)
				require.Equal(t, 2*time.Second, selection.WaitPlan.Timeout)
				require.Empty(t, released)
				if state == "escape_disabled" || state == "owner_bound" || state == "incompatible" {
					require.NotContains(t, acquired, int64(2))
				}
			}
			cache, ok := svc.cache.(*schedulerTestGatewayCache)
			require.True(t, ok)
			require.Equal(t, int64(1), cache.sessionBindings["openai:spillover"])
		})
	}
}

func TestResponsesStickySpillover_AdvancedPropagatesBindingPreservation(t *testing.T) {
	groupID := int64(48204)
	for _, state := range []string{"spillover", "all_busy", "guardian_parent", "task_owner"} {
		t.Run(state, func(t *testing.T) {
			svc := responsesStickySpilloverService(responsesStickySpilloverAccounts(groupID), true, schedulerTestConcurrencyCache{
				acquireResults: map[int64]bool{1: state == "guardian_parent", 2: state != "all_busy"},
				loadMap:        map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 100, CurrentConcurrency: 1}, 2: {AccountID: 2}},
			})
			req := OpenAIAccountScheduleRequest{
				GroupID: &groupID, Platform: PlatformOpenAI, SessionHash: "spillover", StickyAccountID: 1,
				RequestedModel: "gpt-5.1", RequiredCapability: OpenAIEndpointCapabilityResponses,
			}
			if state == "guardian_parent" {
				req.GuardianParentAccountID = 1
			}
			if state == "task_owner" {
				req.DisableStickyEscape = true
				req.PreserveStickyBinding = true
			}
			scheduler := &defaultOpenAIAccountScheduler{service: svc, stats: newOpenAIAccountRuntimeStats()}
			selection, _, err := scheduler.Select(context.Background(), req)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.True(t, selection.PreserveStickyBinding, "保留绑定策略必须传递给抢槽后准入与绑定路径")
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			cache, ok := svc.cache.(*schedulerTestGatewayCache)
			require.True(t, ok)
			require.Equal(t, int64(1), cache.sessionBindings["openai:spillover"])
		})
	}
}
