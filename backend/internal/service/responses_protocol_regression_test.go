package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 从最终计时快照检查协议边界，重构前后使用同一组期望。
func TestResponsesOutputTiming_ProtocolBoundaryRegression(t *testing.T) {
	for _, tc := range []struct {
		name, payload, eventType, kind, status string
		json, token                            bool
	}{
		{"explicit event wins", `{"type":"response.failed","delta":"answer"}`, "response.output_text.delta", "text", CompletionStatusInterrupted, false, true},
		{"explicit unknown is not body fallback", `{"type":"response.output_text.delta","delta":"answer"}`, "unknown", "", CompletionStatusUnknown, false, false},
		{"malformed with explicit event", `{"delta":`, "response.output_text.delta", "", CompletionStatusUnknown, false, false},
		{"DONE is not completion", " [DONE] ", "response.completed", "", CompletionStatusUnknown, false, false},
		{"non-string delta", `{"type":"response.output_text.delta","delta":1}`, "", "", CompletionStatusInterrupted, false, false},
		{"part audio", `{"type":"response.content_part.added","part":{"type":"output_audio","data":"bytes"}}`, "", "audio", CompletionStatusInterrupted, false, false},
		{"item content order", `{"type":"response.output_item.added","item":{"type":"message","content":[{"type":"output_image","data":"image"},{"type":"output_text","text":"answer"}]}}`, "", "image", CompletionStatusInterrupted, false, true},
		{"terminal media without aggregate token", `{"type":"response.completed","response":{"output":[{"type":"message","content":[{"type":"output_audio","data":"bytes"},{"type":"output_text","text":"aggregate"}]}]}}`, "", "audio", CompletionStatusCompleted, false, false},
		{"JSON content order", `{"status":"completed","output":[{"type":"message","content":[{"type":"output_audio","data":"bytes"},{"type":"output_text","text":"answer"}]}]}`, "", "audio", CompletionStatusCompleted, true, true},
		{"compaction completes before envelope status", `{"object":"response.compaction","status":"failed","output":[{"type":"compaction","encrypted_content":"result"}]}`, "", "compaction", CompletionStatusCompleted, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Unix(100, 0)
			o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
			defer o.stop()
			if tc.json {
				o.observeJSON([]byte(tc.payload), start.Add(7*time.Millisecond))
			} else {
				o.observeEvent([]byte(tc.payload), tc.eventType, start.Add(7*time.Millisecond))
			}
			got := o.snapshot(false)
			require.Equal(t, tc.status, got.CompletionStatus)
			if tc.kind == "" {
				require.Nil(t, got.FirstOutputMs)
				require.Nil(t, got.FirstOutputKind)
			} else {
				require.Equal(t, tc.kind, *got.FirstOutputKind)
				require.Equal(t, 7, *got.FirstOutputMs)
			}
			if tc.token {
				require.Equal(t, 7, *got.StrictFirstTokenMs)
				require.Equal(t, 7, *got.LastTokenMs)
			} else {
				require.Nil(t, got.StrictFirstTokenMs)
				require.Nil(t, got.LastTokenMs)
			}
		})
	}
}

func TestResponsesOutputTiming_PreStartAudioEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		knownAudio    bool
	}{
		{"audio output before start", `{"type":"response.audio.delta","delta":"bytes"}`, false},
		{"transcript marker before start", `{"type":"response.audio.transcript.delta","delta":"spoken"}`, true},
		{"audio end marker before start", `{"type":"response.audio.done"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Unix(100, 0)
			o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
			defer o.stop()
			o.observeEvent([]byte(tc.payload), "", start.Add(-time.Millisecond))
			o.confirmNoAudio([]byte(`{"response":{"output":[]}}`))
			got := o.snapshot(false)
			require.Nil(t, got.FirstOutputMs)
			require.Nil(t, got.StrictFirstTokenMs)
			if tc.knownAudio {
				require.Nil(t, got.AudioOutputTokens)
			} else {
				require.Equal(t, 0, *got.AudioOutputTokens)
			}
		})
	}
}

func TestResponsesOutputTiming_CancelBeforeInvalidEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(WithServiceStatusRequest(context.Background(), 1))
	defer cancel()
	o := newResponsesOutputTiming(ctx, time.Unix(100, 0), &Account{Platform: PlatformOpenAI})
	defer o.stop()
	cancel()
	o.observeEvent([]byte(`not JSON`), "response.completed", time.Unix(101, 0))
	got := o.snapshot(false)
	require.Equal(t, CompletionStatusClientDisconnected, got.CompletionStatus)
	require.Equal(t, CompletionStatusClientDisconnected, *got.ServiceStatusObservation.TerminalKind)
	require.Nil(t, got.FirstOutputMs)
}
