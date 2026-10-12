package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// responsesProtocolFacts 是单帧的纯解析结果，不保留正文、时钟或请求 owner。
// 输出顺序供计时 owner 确定首次输出；audioSeen 仅表示与时点无关的音频模式标记。
type responsesProtocolFacts struct {
	observed       bool
	audioSeen      bool
	outputs        []responsesProtocolOutput
	terminalStatus string
	terminalReason string
}

type responsesProtocolOutput struct {
	kind  string
	token bool
}

func (facts *responsesProtocolFacts) addOutput(kind string, token bool) {
	facts.outputs = append(facts.outputs, responsesProtocolOutput{kind: kind, token: token})
}

// parseResponsesEvent 只解释完整 JSON 事件；显式事件类型优先，DONE 不证明完成。
func parseResponsesEvent(payload []byte, eventType string) responsesProtocolFacts {
	var facts responsesProtocolFacts
	if strings.TrimSpace(string(payload)) == "[DONE]" {
		return facts
	} // Responses 完成以协议终态为准。
	if !gjson.ValidBytes(payload) {
		return facts
	}
	if eventType == "" {
		eventType = gjson.GetBytes(payload, "type").String()
	}
	if strings.HasPrefix(eventType, "response.") || eventType == "error" {
		facts.observed = true
	}
	switch eventType {
	case "response.output_text.delta", "response.refusal.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			facts.addOutput("text", true)
		}
	case "response.audio.transcript.delta", "response.audio_transcript.delta":
		facts.audioSeen = true
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			facts.addOutput("text", true)
		}
	case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			facts.addOutput("reasoning", true)
		}
	case "response.function_call_arguments.delta", "response.custom_tool_call_input.delta", "response.mcp_call_arguments.delta", "response.code_interpreter_call_code.delta", "response.shell_call_command.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			facts.addOutput("tool", true)
		}
	case "response.shell_call_command.added":
		if nonemptyTimingString(gjson.GetBytes(payload, "command")) {
			facts.addOutput("tool", true)
		}
	case "response.shell_call_output_content.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta.stdout")) || nonemptyTimingString(gjson.GetBytes(payload, "delta.stderr")) {
			facts.addOutput("tool", true)
		}
	case "response.output_item.added":
		facts.observeItem(gjson.GetBytes(payload, "item"), true)
	case "response.output_item.done":
		// 完整 item 不能为流式补造 token；媒体/压缩结果仍是可识别的输出。
		facts.observeItem(gjson.GetBytes(payload, "item"), false)
	case "response.content_part.added":
		facts.observePart(gjson.GetBytes(payload, "part"), true)
	case "response.reasoning_summary_part.added":
		if nonemptyTimingString(gjson.GetBytes(payload, "part.text")) {
			facts.addOutput("reasoning", true)
		}
	case "response.image_generation_call.partial_image":
		if nonemptyTimingString(gjson.GetBytes(payload, "partial_image_b64")) {
			facts.addOutput("image", false)
		}
	case "response.audio.delta", "response.output_audio.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			facts.addOutput("audio", false)
		}
	case "response.audio.done", "response.output_audio.done", "response.audio.transcript.done", "response.audio_transcript.done":
		// 结束标记不形成输出时点，但已知音频模式不能冒充确认纯文本。
		facts.audioSeen = true
	}
	if openAIStreamEventTypeIsTerminal(eventType) {
		for _, item := range gjson.GetBytes(payload, "response.output").Array() {
			facts.observeItem(item, false)
		}
	}
	status := responsesTimingStatus(eventType, gjson.GetBytes(payload, "response.status").String())
	if status != "" {
		reason := ""
		if status == CompletionStatusUpstreamError || status == CompletionStatusInterrupted {
			upstreamError := gjson.GetBytes(payload, "response.error")
			if !upstreamError.IsObject() {
				upstreamError = gjson.GetBytes(payload, "error")
			}
			reason = responsesTimingProviderReason(upstreamError)
		}
		facts.terminalStatus, facts.terminalReason = status, reason
	}
	return facts
}

// 两种 Responses 传输都只采纳原始 error 对象中的数字 HTTP 状态。
// 未知码、字符串或小数状态不能通过客户端状态/错误文本补造归因。
func responsesTimingProviderReason(upstreamError gjson.Result) string {
	status := 0
	for _, field := range []string{"status_code", "status"} {
		value := upstreamError.Get(field)
		if value.Type == gjson.Number && value.Num >= 100 && value.Num <= 599 && value.Num == float64(int(value.Num)) {
			status = int(value.Num)
			break
		}
	}
	return ServiceStatusProviderReason(status, upstreamError.Get("code").String())
}

func nonemptyTimingString(v gjson.Result) bool {
	return v.Type == gjson.String && v.String() != ""
}

func (facts *responsesProtocolFacts) observePart(part gjson.Result, token bool) {
	switch part.Get("type").String() {
	case "output_text", "text", "refusal":
		if token && (nonemptyTimingString(part.Get("text")) || nonemptyTimingString(part.Get("refusal"))) {
			facts.addOutput("text", true)
		}
	case "reasoning_text", "summary_text":
		if token && nonemptyTimingString(part.Get("text")) {
			facts.addOutput("reasoning", true)
		}
	case "output_audio", "audio":
		if nonemptyTimingString(part.Get("data")) || nonemptyTimingString(part.Get("audio")) {
			facts.addOutput("audio", false)
		}
	case "output_image", "image":
		if nonemptyTimingString(part.Get("image_url")) || nonemptyTimingString(part.Get("data")) {
			facts.addOutput("image", false)
		}
	}
}

func (facts *responsesProtocolFacts) observeItem(item gjson.Result, token bool) {
	switch item.Get("type").String() {
	case "message":
		for _, part := range item.Get("content").Array() {
			facts.observePart(part, token)
		}
	case "reasoning":
		if token {
			for _, part := range item.Get("summary").Array() {
				facts.observePart(part, true)
			}
			for _, part := range item.Get("content").Array() {
				facts.observePart(part, true)
			}
		}
	case "function_call", "custom_tool_call", "mcp_call":
		if token && (nonemptyTimingString(item.Get("arguments")) || nonemptyTimingString(item.Get("input"))) {
			facts.addOutput("tool", true)
		}
	case "tool_search_call":
		arguments := item.Get("arguments")
		if token && (nonemptyTimingString(arguments) || (arguments.IsObject() && len(arguments.Map()) > 0)) {
			facts.addOutput("tool", true)
		}
	case "code_interpreter_call":
		if token && nonemptyTimingString(item.Get("code")) {
			facts.addOutput("tool", true)
		}
	case "shell_call":
		if token {
			for _, command := range item.Get("action.commands").Array() {
				if nonemptyTimingString(command) {
					facts.addOutput("tool", true)
					break
				}
			}
		}
	case "image_generation_call":
		if nonemptyTimingString(item.Get("result")) {
			facts.addOutput("image", false)
		}
	case "compaction", "compaction_summary":
		if nonemptyTimingString(item.Get("encrypted_content")) {
			facts.addOutput("compaction", false)
		}
	}
}

// parseResponsesJSON 接受 observeJSON 已验证的完整 JSON，不将 compaction 回显当作生成内容。
func parseResponsesJSON(payload []byte) responsesProtocolFacts {
	var facts responsesProtocolFacts
	compact := gjson.GetBytes(payload, "object").String() == "response.compaction"
	for _, item := range gjson.GetBytes(payload, "output").Array() {
		if compact && item.Get("type").String() != "compaction" && item.Get("type").String() != "compaction_summary" {
			continue
		}
		facts.observeItem(item, true)
	}
	if compact {
		facts.terminalStatus = CompletionStatusCompleted
		return facts
	}
	switch gjson.GetBytes(payload, "status").String() {
	case "completed":
		facts.terminalStatus = CompletionStatusCompleted
	case "failed":
		facts.terminalStatus = CompletionStatusUpstreamError
		facts.terminalReason = responsesTimingProviderReason(gjson.GetBytes(payload, "error"))
	case "incomplete", "cancelled", "canceled":
		facts.terminalStatus, facts.terminalReason = CompletionStatusInterrupted, "unclassified"
	}
	return facts
}
