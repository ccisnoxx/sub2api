<template>
  <section class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900" aria-labelledby="routing-diagnostics-title" data-testid="routing-diagnostics">
    <h3 id="routing-diagnostics-title" class="text-sm font-black tracking-wider text-gray-900 dark:text-white">{{ t(`${prefix}.title`) }}</h3>
    <p v-if="!diagnostics" class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t(`${prefix}.missing`) }}</p>
    <p v-else-if="diagnostics.schema_version !== 1" class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t(`${prefix}.unsupported`) }}</p>
    <template v-else>
      <dl class="mt-4 grid grid-cols-1 gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="field in fields" :key="field.key" class="min-w-0">
          <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t(`${prefix}.${field.key}`) }}</dt>
          <dd class="mt-1 break-words font-medium text-gray-900 dark:text-white" :data-routing-field="field.key">{{ field.value }}</dd>
        </div>
      </dl>
      <p class="mt-3 text-xs leading-relaxed text-gray-500 dark:text-gray-400">{{ t(`${prefix}.attemptHint`) }}</p>
      <p class="mt-2 text-xs leading-relaxed text-gray-500 dark:text-gray-400">{{ t(`${prefix}.coverageHints.${diagnostics.filter_coverage}`) }}</p>
      <h4 class="mt-5 text-xs font-bold text-gray-700 dark:text-gray-200">{{ t(`${prefix}.filterReasons`) }}</h4>
      <p v-if="diagnostics.filter_reasons === null" class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t(`${prefix}.filtersUnknown`) }}</p>
      <p v-else-if="!filterRows.length" class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t(`${prefix}.noFilters`) }}</p>
      <dl v-else class="mt-2 space-y-2">
        <div v-for="row in filterRows" :key="row.code" class="flex items-start justify-between gap-4 text-sm">
          <dt class="min-w-0 break-words text-gray-600 dark:text-gray-300">{{ t(`${prefix}.filters.${row.code}`) }}</dt>
          <dd class="shrink-0 font-mono font-medium text-gray-900 dark:text-white">{{ row.count }}</dd>
        </div>
      </dl>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OpsRoutingDiagnostics } from '@/api/admin/ops'

const props = defineProps<{ diagnostics?: OpsRoutingDiagnostics | null }>()
const { t } = useI18n()
const prefix = 'admin.ops.errorDetail.routingDiagnostics'
const layers = ['channel_pricing', 'previous_response_id', 'guardian_parent', 'session_hash', 'load_balance', 'gateway_pool']
const reasons = ['channel_pricing_restricted', 'sticky_hit', 'wait_plan', 'selection_failed', 'slot_acquired', 'pool_empty', 'candidates_filtered', 'compact_unsupported', 'selection_exhausted', 'account_list_failed', 'gateway_pool_restricted']
// 只渲染冻结白名单的标签；未来或异常机器码不作为自由文本输出。
const filters = ['excluded', 'not_schedulable', 'platform_mismatch', 'account_model_not_owned', 'model_not_supported', 'channel_upstream_restricted', 'runtime_blocked', 'privacy_not_set', 'proxy_stream_quarantined', 'shadow_parent_unhealthy', 'model_rate_limited', 'turn_state_hold', 'quota_auto_pause', 'quota_auto_pause_5h', 'quota_auto_pause_7d', 'quota_auto_pause_retry_after', 'quota_auto_pause_requests', 'quota_auto_pause_tokens', 'quota_auto_reset_pending_5h', 'quota_auto_reset_pending_7d', 'quota_auto_reset_credit_check_5h', 'quota_auto_reset_credit_check_7d', 'profit_threshold', 'profit_invalid_account_rate', 'capability_mismatch', 'transport_incompatible', 'compact_unsupported', 'scheduling_threshold', 'grok_free_quota_soft_gate', 'grok_team_model_rate_limit', 'grok_model_quota_block', 'gateway_pool_duplicate', 'session_limit', 'capacity_limited', 'recheck_rejected']
function label(code: string | null, allowed: string[], group: string): string {
  return code !== null && allowed.includes(code) ? t(`${prefix}.${group}.${code}`) : t(`${prefix}.unknown`)
}
const fields = computed(() => {
  const d = props.diagnostics
  if (!d || d.schema_version !== 1) return []
  const result = [
    { key: 'candidatePool', value: d.candidate_pool ?? t(`${prefix}.unknown`) },
    { key: 'filteredCandidates', value: d.filtered_candidates ?? t(`${prefix}.unknown`) },
    { key: 'coverage', value: t(`${prefix}.coverageValues.${d.filter_coverage}`) },
    { key: 'selectionLayer', value: label(d.selection_layer, layers, 'layers') },
    { key: 'selectionReason', value: label(d.selection_reason, reasons, 'reasons') },
    { key: 'selectionAttempt', value: d.selection_attempt }
  ]
  if (d.turn !== null) result.push({ key: 'turn', value: d.turn })
  return result
})
const filterRows = computed(() => {
  const d = props.diagnostics
  if (!d || d.schema_version !== 1 || d.filter_reasons === null) return []
  return filters.flatMap(code => {
    const count = d.filter_reasons?.[code]
    return typeof count === 'number' && count > 0 ? [{ code, count }] : []
  })
})
</script>
