package service

// responsesUsageAccumulator 只跟踪已接受用量的来源与音频事实，不选择或累加计费 Token。
// 零值表示尚未接受用量；调用者在 responsesOutputTiming.mu 下串行更新和取快照。
type responsesUsageAccumulator struct {
	usageSource       string
	audioOutputTokens *int
	audioSeen         bool
}

// 已接受终态总量的拆分随同替换，缺失/非法拆分不能继承部分值。
// 已见音频的历史事实不被替换清空，明确零值仍可成为可信拆分。
func (a *responsesUsageAccumulator) observeAccepted(facts responsesAcceptedUsageFacts) {
	a.usageSource = facts.usageSource
	if facts.replaceAudio {
		a.audioOutputTokens = nil
	}
	if facts.audioTokensKnown {
		n := facts.audioTokens
		a.audioOutputTokens = &n
		if n > 0 {
			a.markAudioSeen()
		}
	}
}

func (a *responsesUsageAccumulator) markAudioSeen() {
	a.audioSeen = true
}

// confirmNoAudio 只在协议内容允许确认、且本尝试从未见过音频时补齐可信零值。
func (a *responsesUsageAccumulator) confirmNoAudio() {
	if !a.audioSeen && a.audioOutputTokens == nil {
		zero := 0
		a.audioOutputTokens = &zero
	}
}

func (a responsesUsageAccumulator) snapshot() UsageTiming {
	source := a.usageSource
	if source == "" {
		// 零值组件未接受过用量，不能声称 partial 或 final。
		source = UsageSourceUnknown
	}
	return UsageTiming{UsageSource: source, AudioOutputTokens: a.audioOutputTokens}.Clone()
}
