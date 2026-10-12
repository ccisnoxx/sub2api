package service

import "time"

// responsesServiceObservationEmitter 只绑定尝试、上报源候选并交付独立观测快照。
// 终态优先级由分类器决定，请求最终冻结和持久化仍由既有 owner 负责。
// 调用者在 responsesOutputTiming.mu 下串行使用本组件；源请求保留自己的锁。
type responsesServiceObservationEmitter struct {
	source      *serviceStatusRequest
	attempt     uint64
	observation *ServiceStatusObservation
}

func newResponsesServiceObservationEmitter(source *serviceStatusRequest) responsesServiceObservationEmitter {
	e := responsesServiceObservationEmitter{source: source}
	if source != nil {
		e.attempt = source.beginAttempt()
	}
	return e
}

// emitFrozenTerminal 仅由分类器首次冻结成功的分支调用，不建立第二套终态优先级。
func (e *responsesServiceObservationEmitter) emitFrozenTerminal(status string, at time.Time, reason string) {
	if e.source == nil {
		return
	}
	if status == CompletionStatusClientDisconnected {
		reason = "client_cancelled"
	}
	e.observation = e.source.observation("terminal", status, reason, at)
	e.source.observeAttemptCandidate(e.observation, e.attempt)
}

// snapshot 的缺失终态事实不冻结分类器，后续真实终态仍可替换 unknown。
// at 只用于首次缺失事实；重复快照保留原观测时点。
func (e *responsesServiceObservationEmitter) snapshot(at time.Time) *ServiceStatusObservation {
	if e.source == nil {
		return nil
	}
	if e.observation == nil {
		e.observation = e.source.observation("terminal", CompletionStatusUnknown, "terminal_missing", at)
	}
	e.source.observeAttemptCandidate(e.observation, e.attempt)
	return e.observation.Clone()
}
