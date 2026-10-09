package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIUsageTiming_BillingAndSnapshotRemainIndependent(t *testing.T) {
	for _, atomicBilling := range []bool{false, true} {
		for _, subscriptionBilling := range []bool{false, true} {
			t.Run(map[bool]string{false: "legacy", true: "atomic"}[atomicBilling]+map[bool]string{false: "balance", true: "subscription"}[subscriptionBilling], func(t *testing.T) {
				var previous *UsageLog
				var previousAmount float64
				var previousCommand *UsageBillingCommand
				for _, timed := range []bool{false, true} {
					logs := &openAIRecordUsageLogRepoStub{inserted: true}
					users := &openAIRecordUsageUserRepoStub{}
					subs := &openAIRecordUsageSubRepoStub{}
					svc := newOpenAIRecordUsageServiceForTest(logs, users, subs, nil)
					bill := &openAIRecordUsageBillingRepoStub{}
					if atomicBilling {
						svc.usageBillingRepo = bill
					}
					first, last, zero := 3, 13, 0
					complete := false
					kind := "text"
					result := &OpenAIForwardResult{RequestID: "timing-independent", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 1200, OutputTokens: 300, CacheReadInputTokens: 200}, Duration: time.Second}
					if timed {
						result.UsageTiming = UsageTiming{TimingVersion: 1, StrictFirstTokenMs: &first, LastTokenMs: &last, FirstOutputMs: &first, FirstOutputKind: &kind, AudioOutputTokens: &zero, CompletionStatus: CompletionStatusClientDisconnected, IsComplete: &complete, UsageSource: UsageSourceUpstreamFinal}
					}
					key := &APIKey{ID: 2}
					var sub *UserSubscription
					if subscriptionBilling {
						key.GroupID = i64p(4)
						key.Group = &Group{ID: 4, SubscriptionType: SubscriptionTypeSubscription, RateMultiplier: 1.3}
						sub = &UserSubscription{ID: 5}
					}
					require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: key, User: &User{ID: 1}, Account: &Account{ID: 3}, Subscription: sub}))
					require.Equal(t, 1, logs.calls)
					amount := users.lastAmount
					if atomicBilling {
						require.Equal(t, 1, bill.calls)
						amount = bill.lastCmd.BalanceCost + bill.lastCmd.SubscriptionCost
					} else if subscriptionBilling {
						require.Equal(t, 1, subs.incrementCalls)
					} else {
						require.Equal(t, 1, users.deductCalls)
					}
					if previous != nil {
						if atomicBilling {
							require.Equal(t, previousCommand, bill.lastCmd)
						}
						require.Equal(t, previous.InputTokens, logs.lastLog.InputTokens)
						require.Equal(t, previous.OutputTokens, logs.lastLog.OutputTokens)
						require.Equal(t, previous.CacheReadTokens, logs.lastLog.CacheReadTokens)
						require.Equal(t, previous.TotalCost, logs.lastLog.TotalCost)
						require.Equal(t, previous.ActualCost, logs.lastLog.ActualCost)
						require.Equal(t, previous.RateMultiplier, logs.lastLog.RateMultiplier)
						require.Equal(t, previous.BillingType, logs.lastLog.BillingType)
						require.Equal(t, previousAmount, amount)
						require.Equal(t, result.UsageTiming, logs.lastLog.UsageTiming)
						*result.UsageTiming.StrictFirstTokenMs = 99
						*result.UsageTiming.IsComplete = true
						*result.UsageTiming.FirstOutputKind = "audio"
						require.Equal(t, 3, *logs.lastLog.StrictFirstTokenMs)
						require.False(t, *logs.lastLog.IsComplete)
						require.Equal(t, "text", *logs.lastLog.FirstOutputKind)
					}
					previous = logs.lastLog
					previousAmount = amount
					previousCommand = bill.lastCmd
				}
			})
		}
	}
}
