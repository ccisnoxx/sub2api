package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/gjson"
)

// chatOutputTiming 只观察原始 CC 内容；转换器合成的 completed 不参与终态判断。
// choices 的数量来自本次出站请求，不能用首个 choice 的 finish_reason 宣告整个响应完成。
type chatOutputTiming struct {
	*responsesOutputTiming
	expectedChoices int
	choices         map[int]string
	plainOutputSeen bool
	unknownOutput   bool
}

func newChatOutputTiming(ctx context.Context, start time.Time, expectedChoices int) *chatOutputTiming {
	core := newOutputTiming(ctx, start)
	if core == nil {
		return nil
	}
	return &chatOutputTiming{responsesOutputTiming: core, expectedChoices: max(expectedChoices, 1), choices: make(map[int]string)}
}

func chatTimingExpectedChoices(choices []int) int {
	if len(choices) > 0 {
		return max(choices[0], 1)
	}
	return 1
}

func (o *chatOutputTiming) clientDisconnected() {
	if o != nil {
		o.responsesOutputTiming.clientDisconnected()
	}
}

func (o *chatOutputTiming) snapshot(disconnected bool) UsageTiming {
	if o == nil {
		return UsageTiming{}
	}
	return o.responsesOutputTiming.snapshot(disconnected)
}

func (o *chatOutputTiming) stop() {
	if o != nil {
		o.responsesOutputTiming.stop()
	}
}

// 非流式转换只在完整内容就绪后记录输出时点；原始事件仍决定取消、错误和用量归属。
func observeChatBufferedResponsesEvent(timing *responsesOutputTiming, payload []byte, eventType string, at time.Time) {
	if timing == nil {
		return
	}
	timing.mu.Lock()
	defer timing.mu.Unlock()
	if timing.ctx != nil && errors.Is(timing.ctx.Err(), context.Canceled) {
		timing.finishStatus(CompletionStatusClientDisconnected)
	}
	if strings.HasPrefix(eventType, "response.") || eventType == "error" {
		timing.observed = true
	}
	switch eventType {
	case "response.audio.delta", "response.output_audio.delta", "response.audio.done", "response.output_audio.done", "response.audio.transcript.delta", "response.audio_transcript.delta", "response.audio.transcript.done", "response.audio_transcript.done":
		timing.audioSeen = true
	}
	if status := responsesTimingStatus(eventType, gjson.GetBytes(payload, "response.status").String()); status != "" {
		upstreamError := gjson.GetBytes(payload, "response.error")
		if !upstreamError.IsObject() {
			upstreamError = gjson.GetBytes(payload, "error")
		}
		timing.finishStatusAt(status, at, responsesTimingProviderReason(upstreamError))
	}
}

func (o *chatOutputTiming) observePayload(payload []byte, jsonResponse bool, at time.Time) {
	if o == nil || !gjson.ValidBytes(payload) {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ctx != nil && errors.Is(o.ctx.Err(), context.Canceled) {
		o.finishStatus(CompletionStatusClientDisconnected)
	}
	root := gjson.ParseBytes(payload)
	if !root.Get("choices").IsArray() && root.Get("data.choices").IsArray() {
		root = root.Get("data")
	}
	if upstreamError := root.Get("error"); upstreamError.IsObject() {
		o.observed = true
		o.finishStatusAt(CompletionStatusUpstreamError, at, responsesTimingProviderReason(upstreamError))
	}
	choices := root.Get("choices")
	if !choices.IsArray() {
		return
	}
	o.observed = true
	for _, choice := range choices.Array() {
		content := choice.Get("delta")
		if jsonResponse {
			content = choice.Get("message")
		}
		o.observeContent(content, at)
		index := choice.Get("index")
		if index.Type != gjson.Number || index.Num < 0 || index.Num != float64(index.Int()) || index.Int() >= int64(o.expectedChoices) {
			continue
		}
		i := int(index.Int())
		if _, exists := o.choices[i]; !exists {
			o.choices[i] = ""
		}
		if reason := choice.Get("finish_reason"); nonemptyTimingString(reason) && o.choices[i] == "" {
			o.choices[i] = reason.String()
		}
	}
	if status, finished := o.choiceStatus(); finished {
		o.finishStatusAt(status, at, "")
	}
}

func (o *chatOutputTiming) choiceStatus() (string, bool) {
	if len(o.choices) != o.expectedChoices {
		return "", false
	}
	status := CompletionStatusCompleted
	for i := 0; i < o.expectedChoices; i++ {
		switch o.choices[i] {
		case "stop", "tool_calls", "function_call":
		case "length", "content_filter":
			status = CompletionStatusInterrupted
		default:
			return "", false
		}
	}
	return status, true
}

func (o *chatOutputTiming) observeContent(content gjson.Result, at time.Time) {
	if !content.IsObject() {
		o.unknownOutput = true
		return
	}
	for key := range content.Map() {
		switch key {
		case "role", "content", "refusal", "reasoning", "reasoning_content", "tool_calls", "function_call", "audio":
		default:
			o.unknownOutput = true
		}
	}
	text := content.Get("content")
	if nonemptyTimingString(text) || nonemptyTimingString(content.Get("refusal")) {
		o.output("text", true, at)
		o.plainOutputSeen = true
	} else if text.IsArray() {
		for _, part := range text.Array() {
			switch part.Get("type").String() {
			case "text", "output_text", "refusal", "reasoning_text", "summary_text":
				if nonemptyTimingString(part.Get("text")) || nonemptyTimingString(part.Get("refusal")) {
					o.plainOutputSeen = true
				}
				o.observePart(part, true, at)
			case "audio", "output_audio":
				o.audioSeen = true
				o.observePart(part, true, at)
			case "image", "output_image":
				o.observePart(part, true, at)
			case "image_url":
				if nonemptyTimingString(part.Get("image_url.url")) || nonemptyTimingString(part.Get("image_url")) {
					o.output("image", false, at)
				}
			default:
				o.unknownOutput = true
			}
		}
	} else if text.Exists() && text.Type != gjson.Null && text.Type != gjson.String {
		o.unknownOutput = true
	}
	if nonemptyTimingString(content.Get("reasoning_content")) || nonemptyTimingString(content.Get("reasoning")) {
		o.output("reasoning", true, at)
		o.plainOutputSeen = true
	}
	for _, call := range content.Get("tool_calls").Array() {
		if nonemptyTimingString(call.Get("function.arguments")) || nonemptyTimingString(call.Get("custom.input")) {
			o.output("tool", true, at)
			o.plainOutputSeen = true
		}
	}
	if nonemptyTimingString(content.Get("function_call.arguments")) {
		o.output("tool", true, at)
		o.plainOutputSeen = true
	}
	if audio := content.Get("audio"); audio.Exists() && audio.Type != gjson.Null {
		o.audioSeen = true
		if nonemptyTimingString(audio.Get("data")) {
			o.output("audio", false, at)
		}
		if nonemptyTimingString(audio.Get("transcript")) {
			o.output("text", true, at)
		}
	}
}

// raw 必须是既有 parser 实际接受的 usage。CC 每次整体替换用量，拆分同样整体替换。
func (o *chatOutputTiming) observeAcceptedCCUsage(raw gjson.Result) {
	if o == nil || !raw.IsObject() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	_, final := o.choiceStatus()
	o.timing.UsageSource = UsageSourceUpstreamPartial
	if final {
		o.timing.UsageSource = UsageSourceUpstreamFinal
	}
	o.timing.AudioOutputTokens = nil
	value := raw.Get("completion_tokens_details.audio_tokens")
	if !value.Exists() {
		value = raw.Get("output_tokens_details.audio_tokens")
	}
	if value.Type == gjson.Number && value.Num >= 0 && value.Num == float64(value.Int()) {
		n := int(value.Int())
		o.timing.AudioOutputTokens = &n
		if n > 0 {
			o.audioSeen = true
		}
	} else if !value.Exists() && final && o.plainOutputSeen && !o.audioSeen && !o.unknownOutput {
		zero := 0
		o.timing.AudioOutputTokens = &zero
	}
}

func (o *chatOutputTiming) observeJSON(payload []byte, at time.Time) {
	if o == nil {
		return
	}
	o.observePayload(payload, true, at)
	if _, ok := extractOpenAIUsageFromJSONBytes(payload); !ok {
		return
	}
	// 与 extractOpenAIUsageFromJSONBytes 的原始接受顺序一致，包括兼容 data 包装。
	for _, path := range []string{"usage", "response.usage", "data.usage", "data.response.usage"} {
		if raw := gjson.GetBytes(payload, path); raw.IsObject() {
			o.observeAcceptedCCUsage(raw)
			return
		}
	}
}

// 转换链先使用通用 parser，再以 typed terminal usage 覆盖；此处跟随实际最终接受点。
func observeChatResponsesUsage(timing *responsesOutputTiming, payload []byte, event *apicompat.ResponsesStreamEvent, before OpenAIUsage) {
	if timing == nil {
		return
	}
	var raw gjson.Result
	if isOpenAICompatResponsesTerminalEvent(event.Type) {
		if event.Response != nil && event.Response.Usage != nil && gjson.GetBytes(payload, "response.usage").IsObject() {
			raw = gjson.GetBytes(payload, "response.usage")
		} else if event.Usage != nil {
			raw = gjson.GetBytes(payload, "usage")
		}
	}
	if !raw.IsObject() {
		parsed, ok := extractOpenAIUsageFromJSONBytes(payload)
		if !ok || (openAIStreamEventTypeIsTerminal(event.Type) && !openAIUsageHasTokens(&parsed) && openAIUsageHasTokens(&before)) {
			return
		}
		// 部分事件也须沿用 parser 实际接受的对象，不能漏掉其中的持续音频事实。
		for _, path := range []string{"usage", "response.usage", "data.usage", "data.response.usage"} {
			if candidate := gjson.GetBytes(payload, path); candidate.IsObject() {
				raw = candidate
				break
			}
		}
	}
	hasAudioSplit := raw.Get("output_tokens_details.audio_tokens").Exists() || raw.Get("completion_tokens_details.audio_tokens").Exists()
	// typed ResponsesUsage 与通用 parser 都接受 CC 别名；拆分来自同一个已接受对象。
	if !raw.Get("output_tokens_details.audio_tokens").Exists() && raw.Get("completion_tokens_details.audio_tokens").Exists() {
		raw = gjson.Parse(`{"output_tokens_details":{"audio_tokens":` + raw.Get("completion_tokens_details.audio_tokens").Raw + `}}`)
	}
	timing.observeAcceptedUsage(payload, event.Type, raw)
	if !hasAudioSplit && (event.Type == "response.completed" || event.Type == "response.done") {
		timing.confirmNoAudio(payload)
	}
}
