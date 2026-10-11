package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 未采集记录必须显式导出未知事实，并保留新 KIN 图片计费及模型展示合同。
func TestUsageTimingReplay_LegacyDTOAndNewKINFields(t *testing.T) {
	upstream := "gpt-6-sol"
	log := &service.UsageLog{
		Model: "billing-model", RequestedModel: "requested-model", UpstreamResponseModel: &upstream,
		ImageInputTokens: 11, ImageOutputTokens: 17, ImageInputCost: 0.31, ImageOutputCost: 0.47,
	}
	for _, value := range []any{UsageLogFromService(log), UsageLogFromServiceAdmin(log)} {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, json.Unmarshal(raw, &got))
		require.Contains(t, got, "timing_version", "未接入记录也必须保留明确版本边界")
		require.Equal(t, float64(0), got["timing_version"])
		require.Equal(t, "unknown", got["completion_status"])
		require.Equal(t, "unknown", got["usage_source"])
		for _, field := range []string{"strict_first_token_ms", "last_token_ms", "first_output_ms", "first_output_kind", "audio_output_tokens", "is_complete"} {
			require.Contains(t, got, field)
			require.Nil(t, got[field])
		}
		require.Equal(t, "requested-model", got["model"])
		require.Equal(t, float64(11), got["image_input_tokens"])
		require.Equal(t, float64(17), got["image_output_tokens"])
		require.Equal(t, 0.31, got["image_input_cost"])
		require.Equal(t, 0.47, got["image_output_cost"])
	}
}
