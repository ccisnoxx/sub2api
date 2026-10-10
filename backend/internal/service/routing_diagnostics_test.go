package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoutingDiagnosticsRequestSnapshotsAreIndependent(t *testing.T) {
	ctx := WithRoutingDiagnosticsRequest(context.Background(), 2)
	require.Same(t, ctx, EnsureRoutingDiagnosticsRequest(ctx))
	selectionCtx, first := beginRoutingDiagnosticsSelection(ctx)
	first.observePool([]Account{{ID: 1}, {ID: 1}, {ID: 2}})
	first.reject(1, "model_not_supported")
	first.reject(1, "excluded")
	first.pass(2)
	selection, firstSnapshot, err := finishRoutingDiagnosticsSelection(first, &AccountSelectionResult{Acquired: true}, "load_balance", nil)
	require.NoError(t, err)
	require.Equal(t, 2, *firstSnapshot.CandidatePool)
	require.Equal(t, 1, *firstSnapshot.FilteredCandidates)
	require.Equal(t, "complete", firstSnapshot.FilterCoverage)
	require.Equal(t, 2, *firstSnapshot.Turn)
	firstSnapshot.FilterReasons["model_not_supported"] = 99
	*firstSnapshot.CandidatePool = 99
	require.Equal(t, 1, selection.RoutingDiagnostics.FilterReasons["model_not_supported"])
	require.Equal(t, 2, *GetRoutingDiagnostics(selectionCtx).CandidatePool)

	_, second := beginRoutingDiagnosticsSelection(ctx)
	require.Nil(t, GetRoutingDiagnostics(ctx))
	second.outcome("channel_pricing", "channel_pricing_restricted")
	_, secondSnapshot, attached := finishRoutingDiagnosticsSelection(second, nil, "", ErrNoAvailableAccounts)
	require.ErrorIs(t, attached, ErrNoAvailableAccounts)
	require.Equal(t, ErrNoAvailableAccounts.Error(), attached.Error())
	require.Equal(t, 2, secondSnapshot.SelectionAttempt)
	require.Nil(t, secondSnapshot.CandidatePool)
	require.Nil(t, secondSnapshot.FilteredCandidates)
	require.Nil(t, secondSnapshot.FilterReasons)
	require.Equal(t, "unobserved", secondSnapshot.FilterCoverage)
	require.Equal(t, 1, selection.RoutingDiagnostics.SelectionAttempt)

	wrapped := fmt.Errorf("外层: %w", attached)
	require.Equal(t, secondSnapshot, RoutingDiagnosticsFromError(wrapped))
	var cause *routingDiagnosticsError
	require.True(t, errors.As(wrapped, &cause))
	copy := RoutingDiagnosticsFromError(wrapped)
	*copy.SelectionReason = "changed"
	require.Equal(t, "channel_pricing_restricted", *RoutingDiagnosticsFromError(wrapped).SelectionReason)

	newTurn := WithRoutingDiagnosticsRequest(ctx, 3)
	require.Nil(t, GetRoutingDiagnostics(newTurn))
	_, third := beginRoutingDiagnosticsSelection(newTurn)
	require.Equal(t, 1, third.diagnostics.SelectionAttempt)
	require.Equal(t, 3, *third.diagnostics.Turn)
	require.Equal(t, secondSnapshot, GetRoutingDiagnostics(ctx))
}

func TestRoutingDiagnosticsOwnerRebindPreservesBusinessContext(t *testing.T) {
	type pricingKey struct{}
	business, cancelBusiness := context.WithCancel(context.WithValue(context.Background(), pricingKey{}, "pricing"))
	ownerBase, cancelOwner := context.WithCancel(context.Background())
	ownerCtx := WithRoutingDiagnosticsRequest(ownerBase, 4)
	bound := WithRoutingDiagnosticsOwner(business, ownerCtx)
	require.Equal(t, "pricing", bound.Value(pricingKey{}))
	cancelOwner()
	require.NoError(t, bound.Err())
	_, b := beginRoutingDiagnosticsSelection(bound)
	_, snapshot, _ := finishRoutingDiagnosticsSelection(b, &AccountSelectionResult{Acquired: true}, "session_hash", nil)
	require.Equal(t, 4, *snapshot.Turn)
	require.Equal(t, snapshot, GetRoutingDiagnostics(ownerCtx))
	cancelBusiness()
	require.ErrorIs(t, bound.Err(), context.Canceled)
	require.Same(t, business, WithRoutingDiagnosticsOwner(business, context.Background()))
}

func TestRoutingDiagnosticsConcurrentSequenceAndPublish(t *testing.T) {
	ctx := WithRoutingDiagnosticsRequest(context.Background(), 0)
	const count = 32
	builders := make(chan *routingDiagnosticsBuilder, count)
	var wg sync.WaitGroup
	for range count {
		wg.Go(func() {
			_, b := beginRoutingDiagnosticsSelection(ctx)
			builders <- b
		})
	}
	wg.Wait()
	close(builders)
	seen := make(map[int]bool)
	for b := range builders {
		seen[b.diagnostics.SelectionAttempt] = true
		_, _, err := finishRoutingDiagnosticsSelection(b, nil, "load_balance", ErrNoAvailableAccounts)
		require.ErrorIs(t, err, ErrNoAvailableAccounts)
	}
	require.Len(t, seen, count)
	require.Equal(t, count, GetRoutingDiagnostics(ctx).SelectionAttempt)
	require.Nil(t, GetRoutingDiagnostics(ctx).Turn)
}

func TestRoutingDiagnosticsNullableJSONAndPartialCoverage(t *testing.T) {
	ctx := WithRoutingDiagnosticsRequest(context.Background(), 0)
	_, b := beginRoutingDiagnosticsSelection(ctx)
	b.observePool([]Account{{ID: 1}, {ID: 2}})
	b.reject(1, "unknown_provider_string")
	b.recheckRejected(1, "compact_unsupported")
	b.pass(1) // 同轮 stale compact 恢复，暂缓拒绝不得计数。
	_, snapshot, _ := finishRoutingDiagnosticsSelection(b, &AccountSelectionResult{Acquired: true}, "load_balance", nil)
	require.Equal(t, "partial", snapshot.FilterCoverage)
	require.Equal(t, 0, *snapshot.FilteredCandidates)
	require.Empty(t, snapshot.FilterReasons)
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Len(t, fields, 9)
	require.Contains(t, fields, "turn")
	require.Nil(t, fields["turn"])
	field, ok := reflect.TypeFor[AccountSelectionResult]().FieldByName("RoutingDiagnostics")
	require.True(t, ok)
	require.Equal(t, "-", field.Tag.Get("json"))
	decisionJSON, err := json.Marshal(OpenAIAccountScheduleDecision{RoutingDiagnostics: snapshot})
	require.NoError(t, err)
	require.NotContains(t, string(decisionJSON), "routing_diagnostics")
}
