//go:build unit

package repository

import (
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsInsertErrorLogArgsPreservesExplicitZeroUpstreamStatus(t *testing.T) {
	zero := 0
	args := opsInsertErrorLogArgs(&service.OpsInsertErrorLogInput{UpstreamStatusCode: &zero})

	require.Len(t, args, 39)
	encoded, ok := args[27].(sql.NullInt64)
	require.True(t, ok)
	require.True(t, encoded.Valid)
	require.Zero(t, encoded.Int64)
}

func TestOpsNullableIntPointerDistinguishesNilZeroAndStatus(t *testing.T) {
	missing := opsNullableIntPointer(nil).(sql.NullInt64)
	require.False(t, missing.Valid)

	zeroValue := 0
	zero := opsNullableIntPointer(&zeroValue).(sql.NullInt64)
	require.True(t, zero.Valid)
	require.Zero(t, zero.Int64)

	statusValue := 503
	status := opsNullableIntPointer(&statusValue).(sql.NullInt64)
	require.True(t, status.Valid)
	require.EqualValues(t, 503, status.Int64)
}

func TestOpsInsertErrorLogArgsPreservesSerializedRoutingDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name  string
		raw   *string
		valid bool
	}{
		{name: "SQL NULL"},
		{name: "JSON null", raw: opsRoutingArgsString("null"), valid: true},
		{name: "observed zero", raw: opsRoutingArgsString(`{"schema_version":1,"turn":null,"selection_attempt":1,"selection_layer":"load_balance","selection_reason":"pool_empty","candidate_pool":0,"filtered_candidates":0,"filter_reasons":{},"filter_coverage":"complete"}`), valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := opsInsertErrorLogArgs(&service.OpsInsertErrorLogInput{RoutingDiagnosticsJSON: tc.raw})
			require.Len(t, args, 39)
			encoded, ok := args[38].(sql.NullString)
			require.True(t, ok)
			require.Equal(t, tc.valid, encoded.Valid)
			if tc.raw != nil {
				require.Equal(t, *tc.raw, encoded.String)
			}
		})
	}
}

func opsRoutingArgsString(value string) *string { return &value }
