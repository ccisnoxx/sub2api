package service

import "time"

// responsesOutputTimingTracker 只持有输出时点；协议解析、终态和音频用量由其他 owner 负责。
// start 沿用已有转发起点；调用者在 responsesOutputTiming.mu 下串行更新和取快照。
type responsesOutputTimingTracker struct {
	start              time.Time
	firstOutputMs      *int
	firstOutputKind    *string
	strictFirstTokenMs *int
	lastTokenMs        *int
}

// observe 返回本次输出时点是否被接受，供调用者保留有效音频输出的事实。
func (t *responsesOutputTimingTracker) observe(output responsesProtocolOutput, at time.Time) bool {
	ms := int(at.Sub(t.start).Milliseconds())
	if ms < 0 {
		return false
	}
	if t.firstOutputMs == nil {
		t.firstOutputMs = &ms
		kind := output.kind
		t.firstOutputKind = &kind
	}
	if output.token {
		if t.strictFirstTokenMs == nil {
			first := ms
			t.strictFirstTokenMs = &first
		}
		last := ms
		t.lastTokenMs = &last
	}
	return true
}

// snapshot 仅交付输出字段的独立副本，不冻结终态或停止后续输出采集。
func (t responsesOutputTimingTracker) snapshot() UsageTiming {
	return UsageTiming{
		FirstOutputMs:      t.firstOutputMs,
		FirstOutputKind:    t.firstOutputKind,
		StrictFirstTokenMs: t.strictFirstTokenMs,
		LastTokenMs:        t.lastTokenMs,
	}.Clone()
}
