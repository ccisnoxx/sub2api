<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import { averageUsageTps, formatUsageTps, usageTpsNote, type UsageTimingRow } from '@/utils/usageTiming'

const props = defineProps<{ row: UsageTimingRow }>()
const { t } = useI18n()

const result = computed(() => averageUsageTps(props.row))
const displayValue = computed(() => formatUsageTps(result.value.value))

const explanation = computed(() =>
  [usageTpsNote(result.value, t), t('usage.tpsDescription')].filter(Boolean).join(' ')
)
</script>

<template>
  <span class="inline-flex items-center text-gray-400 dark:text-gray-500">
    <span data-testid="usage-tps-label">{{ t('usage.latencyTps') }}</span>
    <HelpTooltip trigger="click">
      <template #trigger>
        <button
          data-testid="usage-tps-details"
          type="button"
          class="flex h-4 w-4 items-center justify-center rounded-full bg-gray-100 text-gray-400 hover:bg-blue-100 hover:text-blue-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:bg-gray-700 dark:text-gray-500 dark:hover:bg-blue-900/50 dark:hover:text-blue-400"
          :aria-label="t('usage.averageOutputTps')"
        >
          <Icon name="infoCircle" size="xs" aria-hidden="true" />
        </button>
      </template>
      <span class="block pr-5">{{ explanation }}</span>
    </HelpTooltip>
  </span>
  <span
    data-testid="usage-tps-value"
    class="whitespace-nowrap font-medium tabular-nums"
    :class="result.value == null ? 'text-gray-400 dark:text-gray-500' : 'text-cyan-600 dark:text-cyan-400'"
  >
    {{ displayValue }}
    <span v-if="result.partial" data-testid="usage-partial-response" class="ml-1 text-[10px] text-amber-600 dark:text-amber-400">{{ t('usage.partialResponse') }}</span>
  </span>
</template>
