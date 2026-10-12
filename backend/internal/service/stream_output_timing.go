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
// 输出字段由 outputTiming 持有；锁用于与请求取消通知排序。
type responsesOutputTiming struct {
	mu           sync.Mutex
	ctx          context.Context
	timing       UsageTiming // 时序版本与观测元数据，输出和已接受用量由独立组件持有。
	outputTiming responsesOutputTimingTracker
	usage        responsesUsageAccumulator
	completion   responsesCompletionClassifier
	observed     bool
	stopCancel   func() bool
	source       *serviceStatusRequest
	attempt      uint64
}

func newResponsesOutputTiming(ctx context.Context, start time.Time, account *Account) *responsesOutputTiming {
	if start.IsZero() || account == nil || account.Platform != PlatformOpenAI {
		return nil
	}
	o := &responsesOutputTiming{ctx: ctx, outputTiming: responsesOutputTimingTracker{start: start}, timing: UsageTiming{TimingVersion: 1}}
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
	if facts.audioSeen {
		o.usage.markAudioSeen()
	}
	for _, output := range facts.outputs {
		if o.outputTiming.observe(output, at) && output.kind == "audio" {
			o.usage.markAudioSeen()
		}
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
	o.usage.observeAccepted(parseResponsesAcceptedUsage(payload, eventType, raw))
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
	if o == nil || !responsesCanConfirmNoAudio(payload) {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.usage.confirmNoAudio()
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
	output := o.outputTiming.snapshot()
	timing.FirstOutputMs, timing.FirstOutputKind = output.FirstOutputMs, output.FirstOutputKind
	timing.StrictFirstTokenMs, timing.LastTokenMs = output.StrictFirstTokenMs, output.LastTokenMs
	usage := o.usage.snapshot()
	timing.UsageSource, timing.AudioOutputTokens = usage.UsageSource, usage.AudioOutputTokens
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
