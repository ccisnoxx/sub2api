package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesProtocolParser_StreamFacts(t *testing.T) {
	for _, tc := range []struct {
		name, payload, eventType string
		want                     responsesProtocolFacts
	}{
		{"empty transcript marks audio mode", `{"type":"response.audio_transcript.delta","delta":""}`, "", responsesProtocolFacts{observed: true, audioSeen: true}},
		{"reasoning item output order", `{"type":"response.output_item.added","item":{"type":"reasoning","summary":[{"type":"summary_text","text":"summary"}],"content":[{"type":"reasoning_text","text":"thought"}]}}`, "", responsesProtocolFacts{observed: true, outputs: []responsesProtocolOutput{{kind: "reasoning", token: true}, {kind: "reasoning", token: true}}}},
		{"terminal only exposes media and compaction", `{"type":"response.done","response":{"output":[{"type":"function_call","arguments":"aggregate"},{"type":"image_generation_call","result":"image"},{"type":"compaction_summary","encrypted_content":"compact"}]}}`, "", responsesProtocolFacts{observed: true, outputs: []responsesProtocolOutput{{kind: "image"}, {kind: "compaction"}}, terminalStatus: CompletionStatusCompleted}},
		{"nested provider error takes priority", `{"type":"response.failed","response":{"error":{"status_code":503}},"error":{"status_code":429}}`, "", responsesProtocolFacts{observed: true, terminalStatus: CompletionStatusUpstreamError, terminalReason: "provider_5xx"}},
		{"non-object nested error uses top-level error", `{"type":"response.failed","response":{"error":"unknown"},"error":{"code":"slow_down"}}`, "", responsesProtocolFacts{observed: true, terminalStatus: CompletionStatusUpstreamError, terminalReason: "provider_capacity"}},
		{"explicit completed type respects incomplete status", `{"type":"response.failed","response":{"status":"incomplete"}}`, "response.completed", responsesProtocolFacts{observed: true, terminalStatus: CompletionStatusInterrupted, terminalReason: "unclassified"}},
		{"invalid JSON never becomes protocol evidence", `{"type":"response.completed"`, "response.completed", responsesProtocolFacts{}},
		{"DONE is not protocol evidence", "[DONE]", "response.completed", responsesProtocolFacts{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, parseResponsesEvent([]byte(tc.payload), tc.eventType))
		})
	}
}

func TestResponsesProtocolParser_JSONFacts(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		want          responsesProtocolFacts
	}{
		{"JSON observes full text without stream evidence", `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]}]}`, responsesProtocolFacts{outputs: []responsesProtocolOutput{{kind: "text", token: true}}, terminalStatus: CompletionStatusCompleted}},
		{"compaction excludes echoed content and wins status", `{"object":"response.compaction","status":"failed","output":[{"type":"message","content":[{"type":"text","text":"echo"}]},{"type":"compaction","encrypted_content":"compact"}]}`, responsesProtocolFacts{outputs: []responsesProtocolOutput{{kind: "compaction"}}, terminalStatus: CompletionStatusCompleted}},
		{"JSON cancelled status is interrupted", `{"status":"canceled"}`, responsesProtocolFacts{terminalStatus: CompletionStatusInterrupted, terminalReason: "unclassified"}},
		{"JSON failure only uses direct error", `{"status":"failed","error":{"status_code":429},"response":{"error":{"status_code":503}}}`, responsesProtocolFacts{terminalStatus: CompletionStatusUpstreamError, terminalReason: "provider_capacity"}},
		{"unknown status does not prove completion", `{"status":"other"}`, responsesProtocolFacts{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, parseResponsesJSON([]byte(tc.payload)))
		})
	}
}
