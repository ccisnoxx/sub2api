package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesCompletionClassifier_FirstTerminalAndSnapshot(t *testing.T) {
	for _, tc := range []struct{ first, later string }{
		{CompletionStatusCompleted, CompletionStatusClientDisconnected},
		{CompletionStatusClientDisconnected, CompletionStatusCompleted},
		{CompletionStatusUpstreamError, CompletionStatusCompleted},
		{CompletionStatusInterrupted, CompletionStatusUpstreamError},
	} {
		t.Run(tc.first, func(t *testing.T) {
			var classifier responsesCompletionClassifier
			status, complete := classifier.snapshot()
			require.Equal(t, CompletionStatusUnknown, status)
			require.Nil(t, complete)
			require.True(t, classifier.freeze(tc.first))
			require.False(t, classifier.freeze(tc.later))
			status, complete = classifier.snapshot()
			require.Equal(t, tc.first, status)
			require.Equal(t, tc.first == CompletionStatusCompleted, *complete)
			*complete = !*complete
			_, next := classifier.snapshot()
			require.Equal(t, tc.first == CompletionStatusCompleted, *next)
		})
	}
}

func TestResponsesCompletionClassifier_ProtocolStatus(t *testing.T) {
	for _, tc := range []struct{ event, declared, want string }{
		{"response.output_text.delta", "completed", ""},
		{"response.failed", "completed", CompletionStatusUpstreamError},
		{"response.completed", "failed", CompletionStatusUpstreamError},
		{"response.done", "incomplete", CompletionStatusInterrupted},
		{"response.cancelled", "", CompletionStatusInterrupted},
		{"response.canceled", "", CompletionStatusInterrupted},
		{"response.completed", "", CompletionStatusCompleted},
	} {
		t.Run(tc.event+"/"+tc.declared, func(t *testing.T) {
			require.Equal(t, tc.want, responsesTimingStatus(tc.event, tc.declared))
		})
	}
}
