package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResponsesServiceObservationEmitter_MissingThenFrozenTerminal(t *testing.T) {
	ctx := WithServiceStatusRequest(context.Background(), 2)
	emitter := newResponsesServiceObservationEmitter(serviceStatusOwner(ctx))
	start := time.Unix(100, 0).UTC()
	missing := emitter.snapshot(start)
	require.Equal(t, CompletionStatusUnknown, *missing.TerminalKind)
	require.Equal(t, "terminal_missing", *missing.ReasonCode)
	require.Equal(t, start, missing.ObservedAt)
	*missing.LogicalTurn = 99
	*missing.TerminalKind = CompletionStatusUpstreamError
	*missing.ReasonCode = "provider_capacity"
	next := emitter.snapshot(start.Add(time.Second))
	require.Equal(t, 2, *next.LogicalTurn)
	require.Equal(t, CompletionStatusUnknown, *next.TerminalKind)
	require.Equal(t, "terminal_missing", *next.ReasonCode)
	require.Equal(t, start, next.ObservedAt)

	emitter.emitFrozenTerminal(CompletionStatusCompleted, start.Add(time.Second), "provider_capacity")
	completed := emitter.snapshot(start.Add(2 * time.Second))
	require.NoError(t, completed.Validate())
	require.Equal(t, next.ObservationKey, completed.ObservationKey)
	require.Equal(t, CompletionStatusCompleted, *completed.TerminalKind)
	require.Nil(t, completed.ReasonCode)
	require.Equal(t, start.Add(time.Second), completed.ObservedAt)
	require.Equal(t, completed, ServiceStatusFinalObservation(ctx, "client_cancelled"))
	require.Equal(t, CompletionStatusUnknown, *next.TerminalKind)
}

func TestResponsesServiceObservationEmitter_LateAttemptCannotReplaceCurrent(t *testing.T) {
	ctx := WithServiceStatusRequest(context.Background(), 1)
	source := serviceStatusOwner(ctx)
	previous := newResponsesServiceObservationEmitter(source)
	current := newResponsesServiceObservationEmitter(source)
	start := time.Unix(100, 0).UTC()
	current.emitFrozenTerminal(CompletionStatusUpstreamError, start, "provider_capacity")
	previous.emitFrozenTerminal(CompletionStatusCompleted, start.Add(time.Second), "")
	previous.snapshot(start.Add(2 * time.Second))
	final := ServiceStatusFinalObservation(ctx, "")
	require.Equal(t, CompletionStatusUpstreamError, *final.TerminalKind)
	require.Equal(t, "provider_capacity", *final.ReasonCode)
	require.Equal(t, start, final.ObservedAt)
	require.Equal(t, current.snapshot(start), final)
}

func TestResponsesServiceObservationEmitter_NoSource(t *testing.T) {
	emitter := newResponsesServiceObservationEmitter(nil)
	at := time.Unix(100, 0).UTC()
	require.Nil(t, emitter.snapshot(at))
	emitter.emitFrozenTerminal(CompletionStatusCompleted, at, "")
	require.Nil(t, emitter.snapshot(at))
}
