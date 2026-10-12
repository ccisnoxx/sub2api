package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 完成冻结不停止 drain 的用量采集，两个快照的可空字段也不能共享可变状态。
func TestResponsesOutputTiming_TerminalFreezePreservesAcceptedUsage(t *testing.T) {
	for _, cancelFirst := range []bool{true, false} {
		name := "completed before cancellation"
		want := CompletionStatusCompleted
		if cancelFirst {
			name = "cancellation before completed"
			want = CompletionStatusClientDisconnected
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(WithServiceStatusRequest(context.Background(), 0))
			defer cancel()
			start := time.Unix(100, 0)
			o := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
			defer o.stop()
			o.observeEvent([]byte(`{"type":"response.output_text.delta","delta":"answer"}`), "", start.Add(time.Millisecond))
			if cancelFirst {
				cancel()
			}
			final := []byte(`{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":2,"output_tokens":3,"output_tokens_details":{"audio_tokens":1}}}}`)
			o.observeEvent(final, "response.completed", start.Add(2*time.Millisecond))
			o.observeUsage(final, "response.completed", OpenAIUsage{})
			if !cancelFirst {
				cancel()
			}
			got := o.snapshot(false)
			require.Equal(t, want, got.CompletionStatus)
			require.Equal(t, !cancelFirst, *got.IsComplete)
			require.Equal(t, UsageSourceUpstreamFinal, got.UsageSource)
			require.Equal(t, 1, *got.AudioOutputTokens)
			require.Equal(t, want, *got.ServiceStatusObservation.TerminalKind)
			require.Equal(t, want, *ServiceStatusFinalObservation(ctx, "provider_capacity").TerminalKind)

			*got.IsComplete = cancelFirst
			*got.AudioOutputTokens = 99
			*got.ServiceStatusObservation.TerminalKind = CompletionStatusUpstreamError
			next := o.snapshot(false)
			require.Equal(t, want, next.CompletionStatus)
			require.Equal(t, !cancelFirst, *next.IsComplete)
			require.Equal(t, 1, *next.AudioOutputTokens)
			require.Equal(t, want, *next.ServiceStatusObservation.TerminalKind)
		})
	}
}
