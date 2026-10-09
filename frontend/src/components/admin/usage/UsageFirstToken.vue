<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import { firstTokenSeverity, LATENCY_TEXT_CLASSES } from '@/utils/latencyHealth'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import {
  displayedFirstTokenMs, formatUsageTimingDuration, hasStrictUsageTiming, strictFirstTokenMs,
  usageCompletionStatus, type UsageTimingRow,
} from '@/utils/usageTiming'

const props = defineProps<{ row: UsageTimingRow }>()
const { t } = useI18n()
const strict = computed(() => hasStrictUsageTiming(props.row))
const firstMs = computed(() => displayedFirstTokenMs(props.row))
const label = computed(() => t(strict.value ? 'usage.firstToken' : 'usage.legacyFirstToken'))
const kinds = ['text', 'reasoning', 'tool', 'image', 'audio', 'compaction']
const outputKind = computed(() => strict.value && kinds.includes(props.row.first_output_kind ?? '')
  ? t(`usage.outputKinds.${props.row.first_output_kind}`) : t('usage.unknown'))
const sources = ['unknown', 'upstream_final', 'upstream_partial']
const source = computed(() => t(`usage.usageSources.${sources.includes(props.row.usage_source ?? '')
  ? props.row.usage_source : 'unknown'}`))
const strictTime = (ms: number | null | undefined): string => strict.value
  ? formatUsageTimingDuration(ms) : t('usage.timingNotCollected')
</script>

<template>
  <span class="inline-flex items-center text-gray-400 dark:text-gray-500">
    <span data-testid="usage-first-token-label">{{ label }}</span>
    <HelpTooltip trigger="click" width-class="w-80">
      <template #trigger>
        <button
          data-testid="usage-timing-details"
          type="button"
          class="flex h-4 w-4 items-center justify-center rounded-full bg-gray-100 text-gray-400 hover:bg-blue-100 hover:text-blue-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:bg-gray-700 dark:text-gray-500 dark:hover:bg-blue-900/50 dark:hover:text-blue-400"
          :aria-label="label"
        >
          <Icon name="infoCircle" size="xs" aria-hidden="true" />
        </button>
      </template>
      <div data-testid="usage-timing-content" class="space-y-2 pr-5">
        <p>{{ t(strict ? 'usage.strictFirstTokenDescription' : 'usage.firstTokenDescription') }}</p>
        <p v-if="strict && resolveUsageRequestType(row) === 'sync'">{{ t('usage.nonStreamingTimingDescription') }}</p>
        <dl class="grid grid-cols-[1fr_max-content] gap-x-3 gap-y-1">
          <dt>{{ t('usage.strictFirstToken') }}</dt><dd>{{ strictTime(strictFirstTokenMs(row)) }}</dd>
          <dt>{{ t('usage.lastToken') }}</dt><dd>{{ strictTime(row.last_token_ms) }}</dd>
          <dt>{{ t('usage.firstOutput') }}</dt><dd>{{ strictTime(row.first_output_ms) }}</dd>
          <dt>{{ t('usage.firstOutputKind') }}</dt><dd>{{ outputKind }}</dd>
          <dt>{{ t('usage.legacyFirstToken') }}</dt><dd>{{ formatUsageTimingDuration(row.first_token_ms) }}</dd>
          <dt>{{ t('usage.latencyDuration') }}</dt><dd>{{ formatUsageTimingDuration(row.duration_ms) }}</dd>
          <dt>{{ t('usage.completionStatus') }}</dt><dd>{{ t(`usage.completionStatuses.${usageCompletionStatus(row)}`) }}</dd>
          <dt>{{ t('usage.isComplete') }}</dt><dd>{{ row.is_complete == null ? t('usage.unknown') : t(row.is_complete ? 'common.yes' : 'common.no') }}</dd>
          <dt>{{ t('usage.usageSource') }}</dt><dd>{{ source }}</dd>
        </dl>
      </div>
    </HelpTooltip>
  </span>
  <span
    data-testid="usage-first-token-value"
    class="font-medium tabular-nums"
    :class="firstMs == null ? 'text-gray-400 dark:text-gray-500' : LATENCY_TEXT_CLASSES[firstTokenSeverity(firstMs)]"
  >{{ formatUsageTimingDuration(firstMs) }}</span>
</template>
