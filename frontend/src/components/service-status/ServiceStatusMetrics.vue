<template>
  <div class="space-y-2 text-sm">
    <dl class="grid grid-cols-2 gap-x-4 gap-y-1">
      <div v-for="key in countKeys" :key="key" class="flex flex-wrap justify-between gap-x-2">
        <dt class="text-gray-500 dark:text-gray-400">{{ t(`admin.serviceStatus.${key}`) }}</dt>
        <dd class="font-medium tabular-nums">{{ metrics[key].toLocaleString() }}</dd>
      </div>
    </dl>
    <div class="flex flex-wrap gap-x-4 gap-y-1 border-t border-gray-100 pt-2 dark:border-dark-700">
      <span>{{ t('admin.serviceStatus.qualified') }}: {{ metrics.qualified.toLocaleString() }}</span>
      <span>{{ t('admin.serviceStatus.successRate') }}: {{ rate(metrics.success_rate) }}</span>
      <span>{{ t('admin.serviceStatus.errorRate') }}: {{ rate(metrics.error_rate) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ServiceStatusMetrics } from '@/api/admin/serviceStatus'

const props = defineProps<{ metrics: ServiceStatusMetrics; unavailable?: boolean }>()
const { t } = useI18n()
const countKeys = ['success', 'failure', 'excluded', 'unknown'] as const
function rate(value: number | null): string {
  return props.unavailable || value === null
    ? t('admin.serviceStatus.unavailableRate')
    : `${(value * 100).toFixed(1)}%`
}
</script>
