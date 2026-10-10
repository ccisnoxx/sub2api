package service

const (
	CompletionStatusUnknown            = "unknown"
	CompletionStatusCompleted          = "completed"
	CompletionStatusClientDisconnected = "client_disconnected"
	CompletionStatusUpstreamError      = "upstream_error"
	CompletionStatusInterrupted        = "interrupted"
	UsageSourceUnknown                 = "unknown"
	UsageSourceUpstreamFinal           = "upstream_final"
	UsageSourceUpstreamPartial         = "upstream_partial"
)

// UsageTiming 属于一次转发尝试或一个已准入 WS turn，与既有计费数据独立。
// 版本 0 不证明新采集口径；可空字段的 nil 表示未观察到，不能当成零或成功。
// 非流式时点表示完整响应内容的实际观察时点，不是上游生成时点。
type UsageTiming struct {
	ServiceStatusObservation *ServiceStatusObservation `json:"-"`
	TimingVersion            int16
	StrictFirstTokenMs       *int
	LastTokenMs              *int
	FirstOutputMs            *int
	FirstOutputKind          *string
	AudioOutputTokens        *int
	CompletionStatus         string
	IsComplete               *bool
	UsageSource              string
}

// Clone 在生命周期 owner 结束时复制字段，使异步用量任务不依赖可变观察器。
func (t UsageTiming) Clone() UsageTiming {
	t.ServiceStatusObservation = t.ServiceStatusObservation.Clone()
	copyInt := func(p *int) *int {
		if p == nil {
			return nil
		}
		v := *p
		return &v
	}
	copyString := func(p *string) *string {
		if p == nil {
			return nil
		}
		v := *p
		return &v
	}
	t.StrictFirstTokenMs = copyInt(t.StrictFirstTokenMs)
	t.LastTokenMs = copyInt(t.LastTokenMs)
	t.FirstOutputMs = copyInt(t.FirstOutputMs)
	t.AudioOutputTokens = copyInt(t.AudioOutputTokens)
	t.FirstOutputKind = copyString(t.FirstOutputKind)
	if t.IsComplete != nil {
		v := *t.IsComplete
		t.IsComplete = &v
	}
	return t
}
