<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import { BILLING_MODE_TOKEN, getDisplayBillingMode } from '@/utils/billingMode'
import { hasImageOutputTokens } from '@/utils/imageUsage'
import { isUsageRequestType, resolveUsageRequestType } from '@/utils/usageRequestType'
import type { UsageLog } from '@/types'

type TpsRow = Pick<UsageLog,
  'output_tokens' | 'duration_ms' | 'billing_mode' | 'image_count' | 'image_output_tokens' |
  'native_compaction_v2' | 'request_type' | 'stream' | 'openai_ws_mode'
>

const props = defineProps<{ row: TpsRow }>()
const { t } = useI18n()

const unavailableReason = computed(() => {
  const row = props.row
  const mode = getDisplayBillingMode(row)
  const requestType = resolveUsageRequestType(row)
  // 图片生成可能按 token 计费，不能只靠计费模式排除；图片输入不影响文本输出速率。
  if (
    (mode && mode !== BILLING_MODE_TOKEN) || row.image_count > 0 || hasImageOutputTokens(row) ||
    row.native_compaction_v2 || ['live', 'probe', 'gwpool_degraded'].includes(requestType) ||
    (row.request_type && !isUsageRequestType(row.request_type))
  ) {
    return t('usage.tpsNotApplicable')
  }
  if (!Number.isFinite(row.output_tokens) || row.output_tokens <= 0) {
    return t('usage.tpsInvalidOutput')
  }
  if (!Number.isFinite(row.duration_ms) || row.duration_ms == null || row.duration_ms <= 0) {
    return t('usage.tpsInvalidDuration')
  }
  return ''
})

const displayValue = computed(() => {
  if (unavailableReason.value) return '—'
  // 分母沿用整条请求的总耗时，包含首 token 等待；历史记录不需要首字耗时。
  const rate = props.row.output_tokens * 1000 / props.row.duration_ms!
  return rate < 0.01 ? '<0.01 tok/s' : `${rate.toFixed(2)} tok/s`
})

const explanation = computed(() =>
  [unavailableReason.value, t('usage.tpsDescription')].filter(Boolean).join(' ')
)
</script>

<template>
  <span class="inline-flex items-center whitespace-nowrap font-medium tabular-nums text-gray-700 dark:text-gray-300">
    <span data-testid="usage-tps-value">{{ displayValue }}</span>
    <HelpTooltip :content="explanation" trigger="click">
      <template #trigger>
        <button
          type="button"
          class="rounded text-gray-400 hover:text-primary-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-gray-500 dark:hover:text-primary-400"
          :aria-label="`${t('usage.averageOutputTps')}: ${explanation}`"
        >
          <Icon name="infoCircle" size="xs" aria-hidden="true" />
        </button>
      </template>
    </HelpTooltip>
  </span>
</template>
