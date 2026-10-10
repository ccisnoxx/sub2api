package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

// responsesOutputTiming 只观察协议事实，不参与用量解析、重试或下游提交。
// start 是现有 duration 的起点；锁仅用于与请求取消通知排序。
type responsesOutputTiming struct {
	mu         sync.Mutex
	start      time.Time
	ctx        context.Context
	timing     UsageTiming
	observed   bool
	audioSeen  bool
	stopCancel func() bool
	source     *serviceStatusRequest
	attempt    uint64
}

func newResponsesOutputTiming(ctx context.Context, start time.Time, account *Account) *responsesOutputTiming {
	if start.IsZero() || account == nil || account.Platform != PlatformOpenAI {
		return nil
	}
	o := &responsesOutputTiming{start: start, ctx: ctx, timing: UsageTiming{TimingVersion: 1, CompletionStatus: CompletionStatusUnknown, UsageSource: UsageSourceUnknown}}
	o.source = serviceStatusOwner(ctx)
	if o.source != nil {
		o.attempt = o.source.beginAttempt()
	}
	if ctx != nil {
		o.stopCancel = context.AfterFunc(ctx, func() {
			if errors.Is(ctx.Err(), context.Canceled) {
				o.clientDisconnected()
			}
		})
	}
	return o
}

func (o *responsesOutputTiming) clientDisconnected() {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.finishStatus(CompletionStatusClientDisconnected)
}

func (o *responsesOutputTiming) finishStatus(status string) {
	o.finishStatusAt(status, time.Now(), "")
}

func (o *responsesOutputTiming) finishStatusAt(status string, at time.Time, reason string) {
	// 首次终态冻结；取消后 drain 到的完成只更新用量来源，不能改写客户端生命周期。
	if o.timing.CompletionStatus != CompletionStatusUnknown {
		return
	}
	o.timing.CompletionStatus = status
	complete := status == CompletionStatusCompleted
	o.timing.IsComplete = &complete
	if o.source != nil {
		if status == CompletionStatusClientDisconnected {
			reason = "client_cancelled"
		}
		o.timing.ServiceStatusObservation = o.source.observation("terminal", status, reason, at)
		o.source.observeAttemptCandidate(o.timing.ServiceStatusObservation, o.attempt)
	}
}

func (o *responsesOutputTiming) output(kind string, token bool, at time.Time) {
	ms := int(at.Sub(o.start).Milliseconds())
	if ms < 0 {
		return
	}
	if o.timing.FirstOutputMs == nil {
		o.timing.FirstOutputMs = &ms
		k := kind
		o.timing.FirstOutputKind = &k
	}
	if kind == "audio" {
		o.audioSeen = true
	}
	if token {
		if o.timing.StrictFirstTokenMs == nil {
			first := ms
			o.timing.StrictFirstTokenMs = &first
		}
		last := ms
		o.timing.LastTokenMs = &last
	}
}

func (o *responsesOutputTiming) observeEvent(payload []byte, eventType string, at time.Time) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ctx != nil && errors.Is(o.ctx.Err(), context.Canceled) {
		o.finishStatus(CompletionStatusClientDisconnected)
	}
	if strings.TrimSpace(string(payload)) == "[DONE]" {
		return
	} // Responses 完成以协议终态为准。
	if !gjson.ValidBytes(payload) {
		return
	}
	if eventType == "" {
		eventType = gjson.GetBytes(payload, "type").String()
	}
	if strings.HasPrefix(eventType, "response.") || eventType == "error" {
		o.observed = true
	}
	switch eventType {
	case "response.output_text.delta", "response.refusal.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			o.output("text", true, at)
		}
	case "response.audio.transcript.delta", "response.audio_transcript.delta":
		o.audioSeen = true
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			o.output("text", true, at)
		}
	case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			o.output("reasoning", true, at)
		}
	case "response.function_call_arguments.delta", "response.custom_tool_call_input.delta", "response.mcp_call_arguments.delta", "response.code_interpreter_call_code.delta", "response.shell_call_command.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			o.output("tool", true, at)
		}
	case "response.shell_call_command.added":
		if nonemptyTimingString(gjson.GetBytes(payload, "command")) {
			o.output("tool", true, at)
		}
	case "response.shell_call_output_content.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta.stdout")) || nonemptyTimingString(gjson.GetBytes(payload, "delta.stderr")) {
			o.output("tool", true, at)
		}
	case "response.output_item.added":
		o.observeItem(gjson.GetBytes(payload, "item"), true, at)
	case "response.output_item.done":
		// 完整 item 不能为流式补造 token；媒体/压缩结果仍是可识别的输出。
		o.observeItem(gjson.GetBytes(payload, "item"), false, at)
	case "response.content_part.added":
		o.observePart(gjson.GetBytes(payload, "part"), true, at)
	case "response.reasoning_summary_part.added":
		if nonemptyTimingString(gjson.GetBytes(payload, "part.text")) {
			o.output("reasoning", true, at)
		}
	case "response.image_generation_call.partial_image":
		if nonemptyTimingString(gjson.GetBytes(payload, "partial_image_b64")) {
			o.output("image", false, at)
		}
	case "response.audio.delta", "response.output_audio.delta":
		if nonemptyTimingString(gjson.GetBytes(payload, "delta")) {
			o.output("audio", false, at)
		}
	case "response.audio.done", "response.output_audio.done", "response.audio.transcript.done", "response.audio_transcript.done":
		// 结束标记不形成输出时点，但已知音频模式不能冒充确认纯文本。
		o.audioSeen = true
	}
	if openAIStreamEventTypeIsTerminal(eventType) {
		for _, item := range gjson.GetBytes(payload, "response.output").Array() {
			o.observeItem(item, false, at)
		}
	}
	status := responsesTimingStatus(eventType, gjson.GetBytes(payload, "response.status").String())
	if status != "" {
		reason := ""
		if status == CompletionStatusUpstreamError || status == CompletionStatusInterrupted {
			code := gjson.GetBytes(payload, "response.error.code").String()
			if code == "" {
				code = gjson.GetBytes(payload, "error.code").String()
			}
			reason = ServiceStatusProviderReason(0, code)
		}
		o.finishStatusAt(status, at, reason)
	}
}

func responsesTimingStatus(eventType, status string) string {
	switch eventType {
	case "error", "response.failed":
		return CompletionStatusUpstreamError
	case "response.incomplete", "response.cancelled", "response.canceled":
		return CompletionStatusInterrupted
	case "response.completed", "response.done":
		switch status {
		case "failed":
			return CompletionStatusUpstreamError
		case "incomplete", "cancelled", "canceled":
			return CompletionStatusInterrupted
		default:
			return CompletionStatusCompleted
		}
	}
	return ""
}

func nonemptyTimingString(v gjson.Result) bool {
	return v.Type == gjson.String && v.String() != ""
}

func (o *responsesOutputTiming) observePart(part gjson.Result, token bool, at time.Time) {
	switch part.Get("type").String() {
	case "output_text", "text", "refusal":
		if token && (nonemptyTimingString(part.Get("text")) || nonemptyTimingString(part.Get("refusal"))) {
			o.output("text", true, at)
		}
	case "reasoning_text", "summary_text":
		if token && nonemptyTimingString(part.Get("text")) {
			o.output("reasoning", true, at)
		}
	case "output_audio", "audio":
		if nonemptyTimingString(part.Get("data")) || nonemptyTimingString(part.Get("audio")) {
			o.output("audio", false, at)
		}
	case "output_image", "image":
		if nonemptyTimingString(part.Get("image_url")) || nonemptyTimingString(part.Get("data")) {
			o.output("image", false, at)
		}
	}
}

func (o *responsesOutputTiming) observeItem(item gjson.Result, token bool, at time.Time) {
	switch item.Get("type").String() {
	case "message":
		for _, part := range item.Get("content").Array() {
			o.observePart(part, token, at)
		}
	case "reasoning":
		if token {
			for _, part := range item.Get("summary").Array() {
				o.observePart(part, true, at)
			}
			for _, part := range item.Get("content").Array() {
				o.observePart(part, true, at)
			}
		}
	case "function_call", "custom_tool_call", "mcp_call":
		if token && (nonemptyTimingString(item.Get("arguments")) || nonemptyTimingString(item.Get("input"))) {
			o.output("tool", true, at)
		}
	case "tool_search_call":
		arguments := item.Get("arguments")
		if token && (nonemptyTimingString(arguments) || (arguments.IsObject() && len(arguments.Map()) > 0)) {
			o.output("tool", true, at)
		}
	case "code_interpreter_call":
		if token && nonemptyTimingString(item.Get("code")) {
			o.output("tool", true, at)
		}
	case "shell_call":
		if token {
			for _, command := range item.Get("action.commands").Array() {
				if nonemptyTimingString(command) {
					o.output("tool", true, at)
					break
				}
			}
		}
	case "image_generation_call":
		if nonemptyTimingString(item.Get("result")) {
			o.output("image", false, at)
		}
	case "compaction", "compaction_summary":
		if nonemptyTimingString(item.Get("encrypted_content")) {
			o.output("compaction", false, at)
		}
	}
}

// observeUsage 在既有解析器调用处使用：仅报告实际保留的 token 数据来源。
func (o *responsesOutputTiming) observeUsage(payload []byte, eventType string, before OpenAIUsage) {
	if o == nil {
		return
	}
	parsed, ok := extractOpenAIUsageFromJSONBytes(payload)
	if !ok {
		return
	}
	raw := gjson.GetBytes(payload, "usage")
	if !raw.IsObject() {
		raw = gjson.GetBytes(payload, "response.usage")
	}
	if !raw.IsObject() {
		return
	}
	terminal := openAIStreamEventTypeIsTerminal(eventType)
	if terminal && !openAIUsageHasTokens(&parsed) && openAIUsageHasTokens(&before) {
		return
	}
	o.observeAcceptedUsage(payload, eventType, raw)
	// 流式入口可能已先观察终态；总量替换后再确认无音频，避免清空可信的零值。
	if eventType == "json" || eventType == "response.completed" || eventType == "response.done" {
		o.confirmNoAudio(payload)
	}
}

// raw 必须是现有计费解析器实际接受的对象，不能在有多个 usage 字段时另选来源。
func (o *responsesOutputTiming) observeAcceptedUsage(payload []byte, eventType string, raw gjson.Result) {
	if o == nil || !raw.IsObject() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.timing.UsageSource = UsageSourceUpstreamPartial
	status := gjson.GetBytes(payload, "response.status").String()
	if status == "" {
		status = gjson.GetBytes(payload, "status").String()
	}
	if (eventType == "response.completed" || eventType == "response.done") && responsesTimingStatus(eventType, status) == CompletionStatusCompleted {
		o.timing.UsageSource = UsageSourceUpstreamFinal
	}
	if eventType == "json" {
		switch status {
		case "completed":
			o.timing.UsageSource = UsageSourceUpstreamFinal
		case "failed", "incomplete", "cancelled", "canceled", "queued", "in_progress":
			o.timing.UsageSource = UsageSourceUpstreamPartial
		default:
			o.timing.UsageSource = UsageSourceUnknown
			if status == "" && gjson.GetBytes(payload, "object").String() == "response.compaction" {
				o.timing.UsageSource = UsageSourceUpstreamFinal
			}
		}
	}
	// 权威总量已被既有解析器接受；拆分须随同替换，缺失/非法不能沿用部分值。
	if eventType == "json" || openAIStreamEventTypeIsTerminal(eventType) {
		o.timing.AudioOutputTokens = nil
	}
	if v := raw.Get("output_tokens_details.audio_tokens"); v.Type == gjson.Number && v.Int() >= 0 && v.Float() == float64(v.Int()) {
		n := int(v.Int())
		o.timing.AudioOutputTokens = &n
		if n > 0 {
			o.audioSeen = true
		}
	}
}

// observeJSON 使用已完整读到的内容时刻；绝不将终态里的内容当成流式首 token。
func (o *responsesOutputTiming) observeJSON(payload []byte, at time.Time) {
	if o == nil || !gjson.ValidBytes(payload) {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ctx != nil && errors.Is(o.ctx.Err(), context.Canceled) {
		o.finishStatus(CompletionStatusClientDisconnected)
	}
	compact := gjson.GetBytes(payload, "object").String() == "response.compaction"
	for _, item := range gjson.GetBytes(payload, "output").Array() {
		if compact && item.Get("type").String() != "compaction" && item.Get("type").String() != "compaction_summary" {
			continue
		}
		o.observeItem(item, true, at)
	}
	if compact {
		o.finishStatusAt(CompletionStatusCompleted, at, "")
	}
	switch gjson.GetBytes(payload, "status").String() {
	case "completed":
		o.finishStatusAt(CompletionStatusCompleted, at, "")
	case "failed":
		o.finishStatusAt(CompletionStatusUpstreamError, at, ServiceStatusProviderReason(0, gjson.GetBytes(payload, "error.code").String()))
	case "incomplete", "cancelled", "canceled":
		o.finishStatusAt(CompletionStatusInterrupted, at, "unclassified")
	}
}

func (o *responsesOutputTiming) confirmNoAudio(payload []byte) {
	if o == nil {
		return
	}
	for _, path := range []string{"usage.output_tokens_details.audio_tokens", "response.usage.output_tokens_details.audio_tokens"} {
		if gjson.GetBytes(payload, path).Exists() {
			return
		}
	}
	output := gjson.GetBytes(payload, "response.output")
	if !output.IsArray() {
		output = gjson.GetBytes(payload, "output")
	}
	if !output.IsArray() {
		return
	}
	for _, item := range output.Array() {
		switch item.Get("type").String() {
		case "message":
			for _, part := range item.Get("content").Array() {
				switch part.Get("type").String() {
				case "output_text", "refusal":
				default:
					return
				}
			}
		case "reasoning", "function_call", "custom_tool_call", "mcp_call", "tool_search_call", "code_interpreter_call", "shell_call", "image_generation_call", "compaction", "compaction_summary":
		default:
			return
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.audioSeen && o.timing.AudioOutputTokens == nil {
		zero := 0
		o.timing.AudioOutputTokens = &zero
	}
}

func (o *responsesOutputTiming) snapshot(clientDisconnected bool) UsageTiming {
	if o == nil {
		return UsageTiming{}
	}
	if o.stopCancel != nil {
		o.stopCancel()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if clientDisconnected || (o.ctx != nil && errors.Is(o.ctx.Err(), context.Canceled)) {
		o.finishStatus(CompletionStatusClientDisconnected)
	}
	if o.timing.CompletionStatus == CompletionStatusUnknown && o.observed {
		reason := "service_transport"
		if o.ctx != nil && errors.Is(o.ctx.Err(), context.DeadlineExceeded) {
			reason = "service_timeout"
		}
		o.finishStatusAt(CompletionStatusInterrupted, time.Now(), reason)
	}
	if o.source != nil {
		if o.timing.ServiceStatusObservation == nil {
			o.timing.ServiceStatusObservation = o.source.observation("terminal", CompletionStatusUnknown, "terminal_missing", time.Now())
		}
		o.source.observeAttemptCandidate(o.timing.ServiceStatusObservation, o.attempt)
	}
	return o.timing.Clone()
}

func responsesTimingStart(starts []time.Time) time.Time {
	if len(starts) > 0 {
		return starts[0]
	}
	return time.Time{}
}
func (o *responsesOutputTiming) stop() {
	if o != nil {
		if o.stopCancel != nil {
			o.stopCancel()
		}
		// 失败返回仍保存内部事实，不能因此制造用量行。
		o.snapshot(false)
	}
}
func (o *responsesOutputTiming) observeBufferedSSE(body string, at time.Time) {
	if o == nil {
		return
	}
	usage := OpenAIUsage{}
	forEachOpenAISSEFrame(body, func(eventType string, data []byte) {
		o.observeEvent(data, eventType, at)
		o.observeUsage(data, eventType, usage)
		parsed, ok := extractOpenAIUsageFromJSONBytes(data)
		if ok {
			if openAIStreamEventTypeIsTerminal(eventType) {
				if openAIUsageHasTokens(&parsed) || !openAIUsageHasTokens(&usage) {
					usage = parsed
				}
			} else {
				mergeOpenAIUsageNonZero(&usage, parsed)
			}
		}
	})
}

// WS 单 turn owner 只采集已绑定 response ID 的事件，辅助帧无 ID 时属于当前 turn。
func responsesTimingMatchesID(current *string, payload []byte, eventType string) bool {
	id := strings.TrimSpace(gjson.GetBytes(payload, "response.id").String())
	if id == "" {
		id = strings.TrimSpace(gjson.GetBytes(payload, "response_id").String())
	}
	if id == "" && isOpenAIWSTerminalEvent(eventType) {
		id = strings.TrimSpace(gjson.GetBytes(payload, "id").String())
	}
	if id == "" {
		return true
	}
	if *current == "" {
		*current = id
	}
	return *current == id
}
