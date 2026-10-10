package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChatOutputTiming_StrictContentAndChoiceTerminals(t *testing.T) {
	start := time.Now()
	o := newChatOutputTiming(context.Background(), start, 2)
	defer o.stop()
	for _, payload := range []string{
		`{"choices":[{"index":0,"delta":{"role":"assistant","content":"","reasoning_content":"","tool_calls":[{"id":"call_1","function":{"name":"run","arguments":""}}]}}]}`,
		`{"choices":[],"usage":{"prompt_tokens":2,"completion_tokens":0}}`,
		`[DONE]`,
	} {
		o.observePayload([]byte(payload), false, start.Add(time.Millisecond))
	}
	require.Nil(t, o.timing.StrictFirstTokenMs)
	require.Nil(t, o.timing.FirstOutputMs)
	o.observePayload([]byte(`{"choices":[{"index":0,"delta":{"reasoning":"thinking"}}]}`), false, start.Add(20*time.Millisecond))
	o.observePayload([]byte(`{"choices":[{"index":1,"delta":{"content":"answer"}}]}`), false, start.Add(40*time.Millisecond))
	o.observePayload([]byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"function":{"arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`), false, start.Add(60*time.Millisecond))
	require.Equal(t, CompletionStatusUnknown, o.timing.CompletionStatus, "另一个 choice 仍未结束")
	o.observeAcceptedCCUsage(gjson.Parse(`{"prompt_tokens":5,"completion_tokens":3}`))
	require.Equal(t, UsageSourceUpstreamPartial, o.timing.UsageSource)
	o.observePayload([]byte(`{"choices":[{"index":1,"delta":{},"finish_reason":"stop"}]}`), false, start.Add(80*time.Millisecond))
	o.observeAcceptedCCUsage(gjson.Parse(`{"prompt_tokens":5,"completion_tokens":7}`))
	o.observePayload([]byte(`[DONE]`), false, start.Add(100*time.Millisecond))
	timing := o.snapshot(false)
	require.Equal(t, 20, *timing.StrictFirstTokenMs)
	require.Equal(t, 60, *timing.LastTokenMs)
	require.Equal(t, "reasoning", *timing.FirstOutputKind)
	require.Equal(t, CompletionStatusCompleted, timing.CompletionStatus)
	require.Equal(t, UsageSourceUpstreamFinal, timing.UsageSource)
	require.Equal(t, 0, *timing.AudioOutputTokens)
	*timing.LastTokenMs = 999
	require.Equal(t, 60, *o.snapshot(false).LastTokenMs, "返回快照不得共享可变指针")
}

func TestChatOutputTiming_UsageReplacementAudioAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		name, delta, partial, final string
		audio                       *int
	}{
		{name: "positive_partial_final_missing", delta: `{"content":"answer"}`, partial: `{"completion_tokens_details":{"audio_tokens":3}}`, final: `{}`},
		{name: "audio_output_final_missing", delta: `{"audio":{"data":"AQ==","transcript":"voice"}}`, final: `{}`},
		{name: "unknown_output_final_missing", delta: `{"content":"answer","video":"opaque"}`, final: `{}`},
		{name: "invalid_final_split", delta: `{"content":"answer"}`, final: `{"completion_tokens_details":{"audio_tokens":-1}}`},
		{name: "partial_split_replaced", delta: `{"content":"answer"}`, partial: `{"completion_tokens_details":{"audio_tokens":3}}`, final: `{"completion_tokens_details":{"audio_tokens":2}}`, audio: intPointer(2)},
		{name: "confirmed_text", delta: `{"content":"answer"}`, final: `{}`, audio: intPointer(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			o := newChatOutputTiming(context.Background(), start, 1)
			defer o.stop()
			o.observePayload([]byte(`{"choices":[{"index":0,"delta":`+tc.delta+`}]}`), false, start.Add(time.Millisecond))
			if tc.partial != "" {
				o.observeAcceptedCCUsage(gjson.Parse(tc.partial))
			}
			o.observePayload([]byte(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`), false, start.Add(2*time.Millisecond))
			o.observeAcceptedCCUsage(gjson.Parse(tc.final))
			require.Equal(t, tc.audio, o.snapshot(false).AudioOutputTokens)
		})
	}
}

func intPointer(value int) *int { return &value }

func TestChatOutputTiming_FirstTerminalAndAttemptIsolation(t *testing.T) {
	for _, reason := range []string{"length", "content_filter", "unknown_vendor_reason"} {
		t.Run(reason, func(t *testing.T) {
			start := time.Now()
			o := newChatOutputTiming(context.Background(), start, 1)
			defer o.stop()
			o.observePayload([]byte(`{"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":"`+reason+`"}]}`), false, start.Add(time.Millisecond))
			require.Equal(t, CompletionStatusInterrupted, o.snapshot(false).CompletionStatus)
		})
	}
	for _, sequence := range []string{"cancel_then_complete", "complete_then_cancel", "error_then_complete"} {
		t.Run(sequence, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			start := time.Now()
			o := newChatOutputTiming(ctx, start, 1)
			defer o.stop()
			want := CompletionStatusCompleted
			if sequence == "cancel_then_complete" {
				cancel()
				want = CompletionStatusClientDisconnected
			}
			if sequence == "error_then_complete" {
				o.observePayload([]byte(`{"error":{"code":"bad_gateway","message":"failed"}}`), false, start.Add(time.Millisecond))
				want = CompletionStatusUpstreamError
			}
			o.observePayload([]byte(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`), false, start.Add(2*time.Millisecond))
			o.observeAcceptedCCUsage(gjson.Parse(`{"prompt_tokens":5,"completion_tokens":7}`))
			cancel()
			require.Equal(t, want, o.snapshot(false).CompletionStatus)
			fresh := newChatOutputTiming(context.Background(), start, 1)
			defer fresh.stop()
			fresh.observePayload([]byte(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`), false, start.Add(3*time.Millisecond))
			require.Nil(t, fresh.snapshot(false).StrictFirstTokenMs)
			require.Equal(t, CompletionStatusCompleted, fresh.snapshot(false).CompletionStatus)
		})
	}
}
