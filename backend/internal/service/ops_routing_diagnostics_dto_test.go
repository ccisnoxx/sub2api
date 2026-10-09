//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func opsRoutingDTOValue[T any](value T) *T { return &value }

func TestOpsRoutingDiagnosticsDTO_AdminDetailRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name        string
		diagnostics *RoutingDiagnostics
	}{
		{name: "unknown"},
		{name: "observed zero", diagnostics: &RoutingDiagnostics{
			SchemaVersion: 1, SelectionAttempt: 1,
			SelectionLayer: opsRoutingDTOValue("load_balance"), SelectionReason: opsRoutingDTOValue("pool_empty"),
			CandidatePool: opsRoutingDTOValue(0), FilteredCandidates: opsRoutingDTOValue(0),
			FilterReasons: map[string]int{}, FilterCoverage: "complete",
		}},
		{name: "unobserved", diagnostics: &RoutingDiagnostics{
			SchemaVersion: 1, SelectionAttempt: 2,
			SelectionLayer: opsRoutingDTOValue("channel_pricing"), SelectionReason: opsRoutingDTOValue("channel_pricing_restricted"),
			FilterCoverage: "unobserved",
		}},
		{name: "partial WS", diagnostics: &RoutingDiagnostics{
			SchemaVersion: 1, Turn: opsRoutingDTOValue(2), SelectionAttempt: 3,
			SelectionLayer: opsRoutingDTOValue("load_balance"), SelectionReason: opsRoutingDTOValue("selection_exhausted"),
			CandidatePool: opsRoutingDTOValue(4), FilteredCandidates: opsRoutingDTOValue(1),
			FilterReasons: map[string]int{"model_not_supported": 1}, FilterCoverage: "partial",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			detail := &OpsErrorLogDetail{OpsErrorLog: OpsErrorLog{ID: 42, Message: "真实故障"}, RoutingDiagnostics: tc.diagnostics}
			raw, err := json.Marshal(detail)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &fields))
			if tc.diagnostics == nil {
				require.NotContains(t, fields, "routing_diagnostics")
			} else {
				var diagnosticFields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(fields["routing_diagnostics"], &diagnosticFields))
				require.Len(t, diagnosticFields, 9, "未知字段必须显式为 null，已观察空映射保留 {}")
				if tc.diagnostics.CandidatePool == nil {
					require.JSONEq(t, "null", string(diagnosticFields["candidate_pool"]))
				} else {
					expected, err := json.Marshal(*tc.diagnostics.CandidatePool)
					require.NoError(t, err)
					require.JSONEq(t, string(expected), string(diagnosticFields["candidate_pool"]))
				}
				if tc.diagnostics.FilterReasons != nil && len(tc.diagnostics.FilterReasons) == 0 {
					require.JSONEq(t, "{}", string(diagnosticFields["filter_reasons"]))
				}
			}
			var decoded OpsErrorLogDetail
			require.NoError(t, json.Unmarshal(raw, &decoded))
			require.Equal(t, detail.RoutingDiagnostics, decoded.RoutingDiagnostics)
			require.Equal(t, detail.ID, decoded.ID)
			require.Equal(t, detail.Message, decoded.Message)
		})
	}
}

func TestOpsRoutingDiagnosticsDTO_ListAndUserWhitelist(t *testing.T) {
	detail := &OpsErrorLogDetail{
		OpsErrorLog:    OpsErrorLog{ID: 42, Phase: "upstream", Type: "upstream_error", Message: "真实故障", Model: "gpt-5"},
		ErrorBody:      "原始脱敏故障正文",
		UpstreamErrors: `[{"routing_diagnostics":{"candidate_pool":4,"filter_reasons":{"model_not_supported":1}}}]`,
		RoutingDiagnostics: &RoutingDiagnostics{
			SchemaVersion: 1, SelectionAttempt: 1, CandidatePool: opsRoutingDTOValue(4),
			FilteredCandidates: opsRoutingDTOValue(1), FilterReasons: map[string]int{"model_not_supported": 1},
			FilterCoverage: "partial",
		},
	}
	for _, tc := range []struct {
		name string
		dto  any
	}{
		{name: "admin list", dto: &OpsErrorLogList{Errors: []*OpsErrorLog{&detail.OpsErrorLog}}},
		{name: "user list", dto: ToUserErrorRequest(&detail.OpsErrorLog)},
		{name: "user detail", dto: ToUserErrorRequestDetail(detail)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.dto)
			require.NoError(t, err)
			for _, internal := range []string{"routing_diagnostics", "candidate_pool", "filter_reasons", "model_not_supported", "upstream_errors"} {
				require.NotContains(t, string(raw), internal)
			}
			require.Contains(t, string(raw), "真实故障")
		})
	}
	user := ToUserErrorRequestDetail(detail)
	require.Equal(t, detail.ErrorBody, user.ErrorBody, "保持原用户详情故障合同")
}
