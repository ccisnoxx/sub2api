package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesUsageParser_AcceptedObjectAndSource(t *testing.T) {
	for _, tc := range []struct {
		name, payload, eventType, raw string
		want                          responsesAcceptedUsageFacts
	}{
		{"accepted object overrides other body usage", `{"response":{"status":"completed","usage":{"output_tokens_details":{"audio_tokens":99}}}}`, "response.completed", `{"output_tokens_details":{"audio_tokens":2}}`, responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamFinal, replaceAudio: true, audioTokens: 2, audioTokensKnown: true}},
		{"nested status wins", `{"status":"completed","response":{"status":"failed"}}`, "json", `{}`, responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamPartial, replaceAudio: true}},
		{"unknown JSON status", `{"status":"future"}`, "json", `{}`, responsesAcceptedUsageFacts{usageSource: UsageSourceUnknown, replaceAudio: true}},
		{"compaction without status", `{"object":"response.compaction"}`, "json", `{}`, responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamFinal, replaceAudio: true}},
		{"fractional audio is not a split", `{}`, "response.completed", `{"output_tokens_details":{"audio_tokens":1.5}}`, responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamFinal, replaceAudio: true}},
		{"explicit zero is known", `{}`, "response.in_progress", `{"output_tokens_details":{"audio_tokens":0}}`, responsesAcceptedUsageFacts{usageSource: UsageSourceUpstreamPartial, audioTokensKnown: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, parseResponsesAcceptedUsage([]byte(tc.payload), tc.eventType, gjson.Parse(tc.raw)))
		})
	}
}

func TestResponsesUsageParser_NoAudioEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		want          bool
	}{
		{"plain text", `{"output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]}]}`, true},
		{"output missing", `{"status":"completed"}`, false},
		{"unknown part", `{"output":[{"type":"message","content":[{"type":"future"}]}]}`, false},
		{"wrapped invalid split", `{"output":[],"data":{"usage":{"input_tokens":1,"output_tokens":1,"output_tokens_details":{"audio_tokens":-1}}}}`, false},
		{"other usage split cannot be ignored", `{"usage":{"input_tokens":1,"output_tokens":1},"response":{"output":[],"usage":{"output_tokens_details":{"audio_tokens":2}}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, responsesCanConfirmNoAudio([]byte(tc.payload)))
		})
	}
}
