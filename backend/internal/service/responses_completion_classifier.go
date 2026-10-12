package service

// responsesCompletionClassifier 只持有首次终态，不解析协议或提交观测。
// 零值表示尚未观察到终态；调用者在 responsesOutputTiming.mu 下串行调用。
type responsesCompletionClassifier struct {
	terminal string
}

// freeze 的输入由协议分类或生命周期 owner 确认为终态。
// 首次终态之后的取消、错误或 drain 完成不能重新排列客户端生命周期。
func (c *responsesCompletionClassifier) freeze(status string) bool {
	if c.terminal != "" {
		return false
	}
	c.terminal = status
	return true
}

func (c responsesCompletionClassifier) pending() bool {
	return c.terminal == ""
}

// snapshot 每次构造独立的可空值，返回结果不会共享组件内部可变状态。
func (c responsesCompletionClassifier) snapshot() (string, *bool) {
	if c.pending() {
		return CompletionStatusUnknown, nil
	}
	complete := c.terminal == CompletionStatusCompleted
	return c.terminal, &complete
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
