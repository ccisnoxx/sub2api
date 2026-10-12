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
	completion responsesCompletionClassifier
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
	o := &responsesOutputTiming{start: start, ctx: ctx, timing: UsageTiming{TimingVersion: 1, UsageSource: UsageSourceUnknown}}
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
	if !o.completion.freeze(status) {
		return
	}
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
	o.applyProtocolFacts(parseResponsesEvent(payload, eventType), at)
}

// applyProtocolFacts 在原有锁内更新状态，解析不能改变取消与首次终态的排序。
func (o *responsesOutputTiming) applyProtocolFacts(facts responsesProtocolFacts, at time.Time) {
	o.observed = o.observed || facts.observed
	o.audioSeen = o.audioSeen || facts.audioSeen
	for _, output := range facts.outputs {
		o.output(output.kind, output.token, at)
	}
	if facts.terminalStatus != "" {
		o.finishStatusAt(facts.terminalStatus, at, facts.terminalReason)
	}
}

// observeUsage 在既有解析器调用处使用：仅报告实际保留的 token 数据来源。
func (o *responsesOutputTiming) observeUsage(payload []byte, eventType string, before OpenAIUsage) {
	if o == nil {
		return
	}
	parsed, raw, ok := extractOpenAIUsageAndRawFromJSONBytes(payload)
	if !ok {
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
	o.applyProtocolFacts(parseResponsesJSON(payload), at)
}

func (o *responsesOutputTiming) confirmNoAudio(payload []byte) {
	if o == nil {
		return
	}
	// 两类既有 parser 的 usage 优先级不同；帧中已有音频拆分时不能凭纯文本猜零。
	for _, path := range []string{"usage.output_tokens_details.audio_tokens", "response.usage.output_tokens_details.audio_tokens"} {
		if gjson.GetBytes(payload, path).Exists() {
			return
		}
	}
	if _, raw, ok := extractOpenAIUsageAndRawFromJSONBytes(payload); ok && raw.Get("output_tokens_details.audio_tokens").Exists() {
		return
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
	if o.completion.pending() && o.observed {
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
	timing := o.timing.Clone()
	timing.CompletionStatus, timing.IsComplete = o.completion.snapshot()
	return timing
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
