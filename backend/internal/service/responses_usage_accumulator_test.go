package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesUsageAccumulator_AcceptedTransitionsAndSnapshot(t *testing.T) {
	var usage responsesUsageAccumulator
	require.Equal(t, UsageSourceUnknown, usage.snapshot().UsageSource)
	require.Nil(t, usage.snapshot().AudioOutputTokens)
	usage.observeAccepted(responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamPartial, audioTokens: 3, audioTokensKnown: true})
	first := usage.snapshot()
	require.Equal(t, 3, *first.AudioOutputTokens)
	*first.AudioOutputTokens = 99
	usage.observeAccepted(responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamPartial})
	require.Equal(t, 3, *usage.snapshot().AudioOutputTokens, "部分帧缺失拆分不能清空旧事实")
	usage.observeAccepted(responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamFinal, replaceAudio: true})
	usage.confirmNoAudio()
	require.Equal(t, UsageSourceUpstreamFinal, usage.snapshot().UsageSource)
	require.Nil(t, usage.snapshot().AudioOutputTokens, "接受终态总量后缺失拆分不继承部分值，也不抹掉已见音频事实")
	usage.observeAccepted(responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamFinal, replaceAudio: true, audioTokensKnown: true})
	require.Equal(t, 0, *usage.snapshot().AudioOutputTokens, "明确零值仍是可信拆分")
	require.Equal(t, 99, *first.AudioOutputTokens, "后续更新不改动已交付副本")
}

func TestResponsesUsageAccumulator_AudioModePreventsGuessedZero(t *testing.T) {
	var plain, audio responsesUsageAccumulator
	plain.confirmNoAudio()
	require.Equal(t, 0, *plain.snapshot().AudioOutputTokens)
	require.Equal(t, UsageSourceUnknown, plain.snapshot().UsageSource, "无音频确认不伪造 accepted usage 来源")
	audio.markAudioSeen()
	audio.confirmNoAudio()
	require.Nil(t, audio.snapshot().AudioOutputTokens)
	audio.observeAccepted(responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamFinal, replaceAudio: true})
	audio.confirmNoAudio()
	require.Nil(t, audio.snapshot().AudioOutputTokens)
}
