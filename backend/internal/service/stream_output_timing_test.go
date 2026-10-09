package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResponsesOutputTiming_StrictEventsAndMedia(t *testing.T) {
	start := time.Unix(100, 0)
	o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
	defer o.stop()
	for _, event := range []string{
		`{"type":"response.created","response":{"id":"r"}}`,
		`{"type":"response.output_text.delta","delta":""}`,
		`{"type":"response.output_item.added","item":{"type":"function_call","name":"exec","arguments":""}}`,
		`{"type":"response.reasoning_summary_text.done","text":"aggregate"}`,
		`{"type":"response.output_item.done","item":{"type":"reasoning","encrypted_content":"signature"}}`,
	} {
		o.observeEvent([]byte(event), "", start.Add(5*time.Millisecond))
	}
	require.Nil(t, o.timing.StrictFirstTokenMs)
	require.Nil(t, o.timing.FirstOutputMs)
	o.observeEvent([]byte(`{"type":"response.image_generation_call.partial_image","partial_image_b64":"image"}`), "", start.Add(10*time.Millisecond))
	o.observeEvent([]byte(`{"type":"response.reasoning_text.delta","delta":"think"}`), "", start.Add(20*time.Millisecond))
	o.observeEvent([]byte(`{"type":"response.custom_tool_call_input.delta","delta":"pwd"}`), "", start.Add(30*time.Millisecond))
	o.observeEvent([]byte(`{"type":"response.output_text.delta","delta":"answer"}`), "", start.Add(40*time.Millisecond))
	terminal := []byte(`{"type":"response.completed","response":{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":2,"output_tokens":3}}}`)
	o.observeEvent(terminal, "", start.Add(50*time.Millisecond))
	o.observeUsage(terminal, "response.completed", OpenAIUsage{})
	o.confirmNoAudio(terminal)
	got := o.snapshot(false)
	require.EqualValues(t, 1, got.TimingVersion)
	require.Equal(t, 10, *got.FirstOutputMs)
	require.Equal(t, "image", *got.FirstOutputKind)
	require.Equal(t, 20, *got.StrictFirstTokenMs)
	require.Equal(t, 40, *got.LastTokenMs)
	require.Equal(t, CompletionStatusCompleted, got.CompletionStatus)
	require.True(t, *got.IsComplete)
	require.Equal(t, UsageSourceUpstreamFinal, got.UsageSource)
	require.Equal(t, 0, *got.AudioOutputTokens)
}

func TestResponsesOutputTiming_TerminalOrdering(t *testing.T) {
	cases := []struct {
		name     string
		events   []string
		cancelAt int
		want     string
	}{
		{"failure then DONE", []string{`{"type":"response.output_text.delta","delta":"partial"}`, `{"type":"response.failed","response":{"status":"failed"}}`, `[DONE]`, `{"type":"response.completed"}`}, -1, CompletionStatusUpstreamError},
		{"cancel before complete", []string{`{"type":"response.output_text.delta","delta":"partial"}`, `{"type":"response.completed"}`}, 1, CompletionStatusClientDisconnected},
		{"complete before cancel", []string{`{"type":"response.completed"}`}, 1, CompletionStatusCompleted},
		{"transport without terminal", []string{`{"type":"response.output_text.delta","delta":"partial"}`}, -1, CompletionStatusInterrupted},
		{"no protocol fact", nil, -1, CompletionStatusUnknown},
		{"incomplete terminal", []string{`{"type":"response.incomplete"}`, `[DONE]`}, -1, CompletionStatusInterrupted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			start := time.Unix(100, 0)
			o := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
			defer o.stop()
			for i, event := range tc.events {
				if i == tc.cancelAt {
					cancel()
				}
				o.observeEvent([]byte(event), "", start.Add(time.Duration(i+1)*time.Millisecond))
			}
			if tc.cancelAt == len(tc.events) {
				cancel()
			}
			got := o.snapshot(false)
			require.Equal(t, tc.want, got.CompletionStatus)
			if tc.want == CompletionStatusUnknown {
				require.Nil(t, got.IsComplete)
			} else {
				require.Equal(t, tc.want == CompletionStatusCompleted, *got.IsComplete)
			}
		})
	}
}

func TestResponsesOutputTiming_AudioUnknownAndUsageSource(t *testing.T) {
	start := time.Unix(100, 0)
	t.Run("audio with no stats stays unknown", func(t *testing.T) {
		o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
		defer o.stop()
		o.observeEvent([]byte(`{"type":"response.audio.delta","delta":"bytes"}`), "", start.Add(time.Millisecond))
		o.confirmNoAudio([]byte(`{"response":{"output":[]}}`))
		got := o.snapshot(false)
		require.Equal(t, "audio", *got.FirstOutputKind)
		require.Nil(t, got.StrictFirstTokenMs)
		require.Nil(t, got.AudioOutputTokens)
	})
	t.Run("audio transcript or end cannot claim zero audio", func(t *testing.T) {
		for _, event := range []string{`{"type":"response.audio.transcript.delta","delta":"spoken"}`, `{"type":"response.audio.done"}`} {
			o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
			o.observeEvent([]byte(event), "", start.Add(time.Millisecond))
			o.confirmNoAudio([]byte(`{"response":{"output":[]}}`))
			require.Nil(t, o.snapshot(false).AudioOutputTokens)
		}
	})
	t.Run("zero final does not relabel retained partial usage", func(t *testing.T) {
		o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
		defer o.stop()
		partial := []byte(`{"type":"response.in_progress","response":{"usage":{"input_tokens":2,"output_tokens":3,"output_tokens_details":{"audio_tokens":1}}}}`)
		o.observeUsage(partial, "response.in_progress", OpenAIUsage{})
		o.observeUsage([]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":0,"output_tokens":0}}}`), "response.completed", OpenAIUsage{InputTokens: 2, OutputTokens: 3})
		got := o.snapshot(false)
		require.Equal(t, UsageSourceUpstreamPartial, got.UsageSource)
		require.Equal(t, 1, *got.AudioOutputTokens)
	})
	t.Run("terminal aggregate cannot manufacture streaming first token", func(t *testing.T) {
		o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
		defer o.stop()
		o.observeEvent([]byte(`{"type":"response.completed","response":{"output":[{"type":"message","content":[{"type":"output_text","text":"full answer"}]}]}}`), "", start.Add(time.Second))
		got := o.snapshot(false)
		require.Nil(t, got.StrictFirstTokenMs)
		require.Nil(t, got.LastTokenMs)
		require.Nil(t, got.FirstOutputMs)
	})
}

func TestResponsesOutputTiming_NonStreamingAndCompatibility(t *testing.T) {
	start := time.Unix(100, 0)
	o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
	defer o.stop()
	o.observeJSON([]byte(`{"status":"completed","output":[{"type":"compaction","encrypted_content":"opaque"},{"type":"message","content":[{"type":"output_text","text":"answer"}]}]}`), start.Add(42*time.Millisecond))
	got := o.snapshot(false)
	require.Equal(t, "compaction", *got.FirstOutputKind)
	require.Equal(t, 42, *got.FirstOutputMs)
	require.Equal(t, 42, *got.StrictFirstTokenMs)
	require.Equal(t, 42, *got.LastTokenMs)
	require.Nil(t, newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformAnthropic}))
	require.Nil(t, newResponsesOutputTiming(context.Background(), time.Time{}, &Account{Platform: PlatformOpenAI}))
	require.Equal(t, UsageTiming{}, (*responsesOutputTiming)(nil).snapshot(false))
}

func TestResponsesOutputTiming_CompactEchoIsNotGeneratedOutput(t *testing.T) {
	start := time.Unix(100, 0)
	o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
	defer o.stop()
	o.observeJSON([]byte(`{"object":"response.compaction","output":[{"type":"message","role":"user","content":[{"type":"text","text":"echoed input"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"earlier answer"}]},{"type":"compaction","encrypted_content":"new opaque result"}]}`), start.Add(time.Second))
	got := o.snapshot(false)
	require.Equal(t, "compaction", *got.FirstOutputKind)
	require.Nil(t, got.StrictFirstTokenMs)
	require.Equal(t, CompletionStatusCompleted, got.CompletionStatus)
}

func TestResponsesOutputTiming_NonemptyWhitespaceAndInvalidAudio(t *testing.T) {
	start := time.Unix(100, 0)
	o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
	defer o.stop()
	o.observeEvent([]byte(`{"type":"response.output_text.delta","delta":" "}`), "", start.Add(time.Millisecond))
	payload := []byte(`{"status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1,"output_tokens_details":{"audio_tokens":-1}}}`)
	o.observeJSON(payload, start.Add(time.Second))
	o.observeUsage(payload, "json", OpenAIUsage{})
	o.confirmNoAudio(payload)
	got := o.snapshot(false)
	require.Equal(t, 1, *got.StrictFirstTokenMs)
	require.Nil(t, got.AudioOutputTokens)
	unknown := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
	defer unknown.stop()
	unknown.observeJSON([]byte(`{"output":[],"usage":{"input_tokens":1,"output_tokens":1}}`), start)
	require.Equal(t, CompletionStatusUnknown, unknown.snapshot(false).CompletionStatus)
}

func TestResponsesOutputTiming_NativeToolContentEvents(t *testing.T) {
	start := time.Unix(100, 0)
	for _, event := range []string{
		`{"type":"response.code_interpreter_call_code.delta","delta":"print(1)"}`,
		`{"type":"response.shell_call_command.added","command":"pwd"}`,
		`{"type":"response.shell_call_command.delta","delta":"pwd"}`,
		`{"type":"response.shell_call_output_content.delta","delta":{"stdout":"/tmp","stderr":""}}`,
		`{"type":"response.output_item.added","item":{"type":"tool_search_call","arguments":{"query":"docs"}}}`,
		`{"type":"response.output_item.added","item":{"type":"code_interpreter_call","code":"print(1)"}}`,
	} {
		o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
		o.observeEvent([]byte(event), "", start.Add(7*time.Millisecond))
		got := o.snapshot(false)
		require.Equal(t, 7, *got.StrictFirstTokenMs, event)
		require.Equal(t, "tool", *got.FirstOutputKind, event)
	}
	o := newResponsesOutputTiming(context.Background(), start, &Account{Platform: PlatformOpenAI})
	defer o.stop()
	for _, event := range []string{
		`{"type":"response.code_interpreter_call.in_progress","item_id":"metadata"}`,
		`{"type":"response.code_interpreter_call_code.done","code":"aggregate"}`,
		`{"type":"response.shell_call_command.added","command":""}`,
		`{"type":"response.shell_call_output_content.delta","delta":{"stdout":"","stderr":""}}`,
		`{"type":"response.shell_call_output_content.done","output":[{"stdout":"aggregate"}]}`,
		`{"type":"response.output_item.done","item":{"type":"tool_search_call","arguments":{"query":"aggregate"}}}`,
	} {
		o.observeEvent([]byte(event), "", start.Add(time.Second))
	}
	require.Nil(t, o.snapshot(false).StrictFirstTokenMs)
}
