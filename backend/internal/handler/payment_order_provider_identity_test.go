//go:build unit

package handler

import (
	"encoding/json"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestSanitizePaymentOrderPreservesProviderIdentityForLegacyMethods(t *testing.T) {
	for _, tc := range []struct {
		name, method, column, snapshot, expected string
	}{
		{"alias Airwallex", "legacy_air_method", "airwallex", "", "airwallex"},
		{"snapshot authority", "legacy_air_method", "stripe", "airwallex", "airwallex"},
		{"Stripe Alipay", "alipay", "stripe", "", "stripe"},
		{"unknown binding", "legacy_air_method", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order := &dbent.PaymentOrder{ID: 42, PaymentType: tc.method, ProviderKey: &tc.column}
			if tc.snapshot != "" {
				order.ProviderSnapshot = map[string]any{"provider_key": tc.snapshot, "merchant_id": "private-merchant"}
			}
			raw, err := json.Marshal(sanitizePaymentOrderForResponse(order))
			require.NoError(t, err)
			var response map[string]any
			require.NoError(t, json.Unmarshal(raw, &response))
			require.Equal(t, tc.expected, response["provider_key"])
			require.Equal(t, tc.method, response["payment_type"])
			require.NotContains(t, response, "provider_snapshot")
			require.NotContains(t, string(raw), "private-merchant")
		})
	}
}
