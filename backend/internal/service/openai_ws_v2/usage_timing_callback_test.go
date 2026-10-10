package openai_ws_v2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRelayUsageTimingCallback_UsesAuthoritativeTurnAndAcceptedUsage(t *testing.T) {
	start := time.Unix(100, 0)
	state := &relayState{}
	state.setPendingTurnStartedAt(start)
	now := start
	var facts []RelayObservedEvent
	observe := func(payload string) {
		observeUpstreamMessage(state, []byte(payload), start, func() time.Time { return now }, nil, func(event RelayObservedEvent) { facts = append(facts, event) })
	}
	observe(`{"type":"response.created","response":{"id":"a"}}`)
	now = start.Add(time.Millisecond)
	observe(`{"type":"response.output_text.delta","response_id":"a","delta":"first"}`)
	secondStart := start.Add(time.Second)
	state.setPendingTurnStartedAt(secondStart)
	now = secondStart
	observe(`{"type":"response.created","response":{"id":"b"}}`)
	now = secondStart.Add(time.Millisecond)
	observe(`{"type":"response.output_text.delta","response_id":"a","delta":"interleaved"}`)
	require.Equal(t, "a", facts[3].ResponseID)
	require.Equal(t, start, facts[3].StartedAt)
	now = secondStart.Add(2 * time.Millisecond)
	observe(`{"type":"response.completed","response":{"id":"a","usage":{"input_tokens":"bad","output_tokens":2}}}`)
	require.False(t, facts[4].UsageAccepted)
	now = secondStart.Add(3 * time.Millisecond)
	observe(`{"type":"response.completed","usage":{"input_tokens":99,"output_tokens":99},"response":{"id":"b","usage":{"input_tokens":3,"output_tokens":4}}}`)
	require.True(t, facts[5].UsageAccepted)
	require.JSONEq(t, `{"input_tokens":3,"output_tokens":4}`, facts[5].AcceptedUsage)
	require.Equal(t, secondStart, facts[5].StartedAt)
	require.Equal(t, now, facts[5].ObservedAt)
}

func TestRelayUsageTimingCallback_ZeroTerminalRetainsProgressiveSource(t *testing.T) {
	start := time.Unix(100, 0)
	state := &relayState{}
	state.setPendingTurnStartedAt(start)
	accepted := []bool{}
	for _, payload := range []string{`{"type":"response.in_progress","response":{"id":"a","usage":{"input_tokens":2,"output_tokens":3}}}`, `{"type":"response.completed","response":{"id":"a","usage":{"input_tokens":0,"output_tokens":0}}}`} {
		observeUpstreamMessage(state, []byte(payload), start, func() time.Time { return start }, nil, func(event RelayObservedEvent) { accepted = append(accepted, event.UsageAccepted) })
	}
	require.Equal(t, []bool{true, false}, accepted)
	require.Equal(t, 2, state.usage.InputTokens)
	require.Equal(t, 3, state.usage.OutputTokens)
}
