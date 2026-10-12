package service

import "github.com/tidwall/gjson"

// responsesAcceptedUsageFacts 是原计费解析器已接受对象的观测事实，不保留正文或 raw 引用。
type responsesAcceptedUsageFacts struct {
	usageSource      string
	replaceAudio     bool
	audioTokens      int
	audioTokensKnown bool
}

// parseResponsesAcceptedUsage 不选择 usage 来源；raw 必须是调用者实际接受的对象。
func parseResponsesAcceptedUsage(payload []byte, eventType string, raw gjson.Result) responsesAcceptedUsageFacts {
	facts := responsesAcceptedUsageFacts{
		usageSource:  UsageSourceUpstreamPartial,
		replaceAudio: eventType == "json" || openAIStreamEventTypeIsTerminal(eventType),
	}
	status := gjson.GetBytes(payload, "response.status").String()
	if status == "" {
		status = gjson.GetBytes(payload, "status").String()
	}
	if (eventType == "response.completed" || eventType == "response.done") && responsesTimingStatus(eventType, status) == CompletionStatusCompleted {
		facts.usageSource = UsageSourceUpstreamFinal
	}
	if eventType == "json" {
		switch status {
		case "completed":
			facts.usageSource = UsageSourceUpstreamFinal
		case "failed", "incomplete", "cancelled", "canceled", "queued", "in_progress":
			facts.usageSource = UsageSourceUpstreamPartial
		default:
			facts.usageSource = UsageSourceUnknown
			if status == "" && gjson.GetBytes(payload, "object").String() == "response.compaction" {
				facts.usageSource = UsageSourceUpstreamFinal
			}
		}
	}
	if v := raw.Get("output_tokens_details.audio_tokens"); v.Type == gjson.Number && v.Int() >= 0 && v.Float() == float64(v.Int()) {
		facts.audioTokens, facts.audioTokensKnown = int(v.Int()), true
	}
	return facts
}

// responsesCanConfirmNoAudio 仅检查协议允许的无音频证据，不读取或更新尝试状态。
func responsesCanConfirmNoAudio(payload []byte) bool {
	// 两类既有 parser 的 usage 优先级不同；帧中已有音频拆分时不能凭纯文本猜零。
	for _, path := range []string{"usage.output_tokens_details.audio_tokens", "response.usage.output_tokens_details.audio_tokens"} {
		if gjson.GetBytes(payload, path).Exists() {
			return false
		}
	}
	if _, raw, ok := extractOpenAIUsageAndRawFromJSONBytes(payload); ok && raw.Get("output_tokens_details.audio_tokens").Exists() {
		return false
	}
	output := gjson.GetBytes(payload, "response.output")
	if !output.IsArray() {
		output = gjson.GetBytes(payload, "output")
	}
	if !output.IsArray() {
		return false
	}
	for _, item := range output.Array() {
		switch item.Get("type").String() {
		case "message":
			for _, part := range item.Get("content").Array() {
				switch part.Get("type").String() {
				case "output_text", "refusal":
				default:
					return false
				}
			}
		case "reasoning", "function_call", "custom_tool_call", "mcp_call", "tool_search_call", "code_interpreter_call", "shell_call", "image_generation_call", "compaction", "compaction_summary":
		default:
			return false
		}
	}
	return true
}
