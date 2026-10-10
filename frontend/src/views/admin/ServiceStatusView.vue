<template>
  <AppLayout>
    <div class="space-y-6 text-gray-900 dark:text-gray-100">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold">{{ t('admin.serviceStatus.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.serviceStatus.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loadingConfig || saving || forbidden || !canAccess" @click="loadConfig">
          {{ t('admin.serviceStatus.reloadConfig') }}
        </button>
      </header>

      <p class="rounded-xl border border-gray-200 bg-gray-100 p-4 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300">
        {{ t('admin.serviceStatus.boundedNotice') }}
      </p>
      <p v-if="message" role="alert" class="card border-amber-300 p-4 text-sm text-amber-800 dark:text-amber-300">{{ t(`admin.serviceStatus.${message}`) }}</p>
      <p v-if="loadingConfig || loadingSnapshot" role="status" class="text-sm text-gray-500">{{ t('admin.serviceStatus.loading') }}</p>

      <section v-if="config && !forbidden" class="card p-4 sm:p-6">
        <div class="flex flex-wrap items-end gap-4">
          <div class="w-full min-w-0 sm:w-auto sm:flex-1 sm:min-w-[180px]">
            <label for="status-platform" class="input-label">{{ t('admin.serviceStatus.platform') }}</label>
            <select id="status-platform" v-model="platform" class="input" :disabled="!config.enabled || saving || conflict">
              <option value="">{{ t('admin.serviceStatus.allPlatforms') }}</option>
              <option v-for="name in config.platforms" :key="name" :value="name">{{ name }}</option>
            </select>
          </div>
          <div class="w-full min-w-0 sm:w-auto sm:flex-1 sm:min-w-[180px]">
            <label for="status-range" class="input-label">{{ t('admin.serviceStatus.range') }}</label>
            <select id="status-range" v-model="range" class="input" :disabled="!config.enabled || saving || conflict">
              <option v-for="value in ranges" :key="value" :value="value">{{ t(`admin.serviceStatus.range${value}`) }}</option>
            </select>
          </div>
          <button type="button" class="btn btn-primary w-full sm:w-auto" :disabled="loadingSnapshot || !config.enabled || saving || conflict" @click="loadSnapshot">
            {{ t('admin.serviceStatus.refresh') }}
          </button>
        </div>
      </section>

      <template v-if="snapshot && !forbidden">
        <section class="card p-4 sm:p-6" data-testid="status-summary">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h2 class="text-lg font-semibold">{{ t('admin.serviceStatus.current') }}</h2>
            <span class="rounded-lg px-3 py-1 font-semibold" :class="healthClass(snapshot.health)">{{ healthLabel(snapshot.health) }}</span>
          </div>
          <p v-if="snapshot.unknown_reason" class="mt-2 text-sm text-gray-500">{{ reasonLabel(snapshot.unknown_reason) }}</p>
          <p v-if="snapshot.enabled_platforms.length === 0" class="mt-2 text-sm">{{ t('admin.serviceStatus.noPlatforms') }}</p>
          <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.serviceStatus.summaryNotice') }}</p>
          <dl class="mt-4 grid gap-4 text-sm sm:grid-cols-2">
            <div><dt class="text-gray-500">{{ t('admin.serviceStatus.current') }} (UTC)</dt><dd>{{ interval(snapshot.window) }}</dd></div>
            <div><dt class="text-gray-500">{{ t('admin.serviceStatus.watermark') }} (UTC)</dt><dd>{{ timestamp(snapshot.observed_through) }}</dd></div>
            <div><dt class="text-gray-500">{{ t('admin.serviceStatus.enabledPlatforms') }}</dt><dd class="break-words">{{ snapshot.enabled_platforms.join(', ') || '—' }}</dd></div>
            <div><dt class="text-gray-500">{{ t('admin.serviceStatus.generatedAt') }} (UTC)</dt><dd>{{ timestamp(snapshot.generated_at) }} · {{ t('admin.serviceStatus.configVersion') }} {{ snapshot.config_version }}</dd></div>
          </dl>
        </section>

        <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" :aria-label="t('admin.serviceStatus.platform')">
          <article v-for="item in snapshot.platforms" :key="item.platform" class="card p-4">
            <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
              <h3 class="font-semibold">{{ item.platform }}</h3>
              <span class="rounded-md px-2 py-1 text-xs" :class="healthClass(item.health)">{{ healthLabel(item.health) }}</span>
            </div>
            <p v-if="item.unknown_reason" class="mb-3 text-xs text-gray-500">{{ reasonLabel(item.unknown_reason) }}</p>
            <ServiceStatusMetrics :metrics="item" />
          </article>
        </section>

        <section class="card p-4 sm:p-6">
          <h2 class="text-lg font-semibold">{{ t('admin.serviceStatus.coverage') }}</h2>
          <p class="mt-2 text-sm text-gray-500">{{ t('admin.serviceStatus.range') }}: {{ interval(snapshot.history_range) }}</p>
          <p v-if="!snapshot.coverage.length" class="mt-2 text-sm text-gray-500">{{ t('admin.serviceStatus.noCoverage') }}</p>
          <ul v-else class="mt-3 max-h-48 space-y-1 overflow-y-auto text-sm" tabindex="0" :aria-label="t('admin.serviceStatus.coverage')">
            <li v-for="(item, index) in snapshot.coverage" :key="index" class="break-words">{{ item.platform }} · {{ interval(item) }}</li>
          </ul>
          <div v-if="snapshot.gaps.length" class="mt-4 rounded-lg bg-gray-100 p-3 text-sm dark:bg-dark-800">
            <h3 class="font-medium">{{ t('admin.serviceStatus.gaps') }}</h3>
            <ul class="mt-2 space-y-1"><li v-for="gap in snapshot.gaps" :key="gap">{{ reasonLabel(gap) }}</li></ul>
          </div>
        </section>

        <section class="card overflow-hidden p-4 sm:p-6">
          <h2 class="mb-4 text-lg font-semibold">{{ t('admin.serviceStatus.scopes') }}</h2>
          <p v-if="!snapshot.scopes.length" class="text-sm text-gray-500">{{ t('admin.serviceStatus.emptyScopes') }}</p>
          <div v-else class="overflow-x-auto" tabindex="0" :aria-label="t('admin.serviceStatus.scopes')">
            <table class="w-full min-w-[740px] text-left text-sm">
              <thead class="text-gray-500"><tr><th class="pb-3 pr-4">{{ t('admin.serviceStatus.group') }} / {{ t('admin.serviceStatus.model') }}</th><th class="pb-3 pr-4">{{ t('admin.serviceStatus.state') }}</th><th class="pb-3">{{ t('admin.serviceStatus.counts') }}</th></tr></thead>
              <tbody>
                <tr v-for="item in snapshot.scopes" :key="scopeKey(item)" class="border-t border-gray-100 dark:border-dark-700">
                  <td class="max-w-[260px] break-words py-4 pr-4 align-top"><div class="font-semibold">{{ item.group_name }} (#{{ item.group_id }})</div><div>{{ item.platform }}</div><div class="mt-1 font-mono">{{ item.requested_model || '—' }}</div></td>
                  <td class="max-w-[260px] py-4 pr-4 align-top"><span class="rounded-md px-2 py-1 text-xs" :class="healthClass(item.health)">{{ healthLabel(item.health) }}</span><p v-if="item.unknown_reason" class="mt-2 text-xs text-gray-500">{{ reasonLabel(item.unknown_reason) }}</p><p v-if="item.incident_phase" class="mt-2 text-xs">{{ phaseLabel(item.incident_phase) }}</p><p class="mt-2 text-xs text-gray-500">{{ t('admin.serviceStatus.lastEvidence') }}: {{ timestamp(item.last_evidence_at) }}</p></td>
                  <td class="min-w-[280px] py-4 align-top"><ServiceStatusMetrics :metrics="item" /></td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="card p-4 sm:p-6">
          <h2 class="text-lg font-semibold">{{ t('admin.serviceStatus.history') }}</h2>
          <p class="mt-1 text-sm text-gray-500">{{ t('admin.serviceStatus.historyNotice') }}</p>
          <p v-if="!snapshot.history.length" class="mt-4 text-sm text-gray-500">{{ t('admin.serviceStatus.emptyHistory') }}</p>
          <ul v-else class="mt-4 space-y-3" data-testid="status-history">
            <li v-for="(point, index) in historyPage" :key="`${scopeKey(point)}:${point.start}:${index}`" class="rounded-lg border p-3" :class="historyUnknown(point) ? 'border-gray-200 bg-gray-100 text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400' : 'border-gray-200 dark:border-dark-700'">
              <div class="flex flex-wrap justify-between gap-2 text-sm"><span class="break-all font-medium">{{ point.platform }} · {{ point.group_name }} (#{{ point.group_id }}) · {{ point.requested_model || '—' }}</span><span>{{ interval(point) }} (UTC)</span></div>
              <p v-if="historyUnknown(point)" class="my-2 text-xs">{{ point.covered ? reasonLabel(point.unknown_reason) : t('admin.serviceStatus.uncovered') }}</p>
              <ServiceStatusMetrics class="mt-2" :metrics="point" :unavailable="historyUnknown(point)" />
            </li>
          </ul>
          <div v-if="historyPages > 1" class="mt-4 flex flex-wrap items-center justify-between gap-3">
            <button type="button" class="btn btn-secondary" :disabled="historyIndex === 0" @click="historyIndex--">{{ t('admin.serviceStatus.previous') }}</button>
            <span class="text-sm">{{ t('admin.serviceStatus.page', { page: historyIndex + 1, total: historyPages }) }}</span>
            <button type="button" class="btn btn-secondary" :disabled="historyIndex + 1 >= historyPages" @click="historyIndex++">{{ t('admin.serviceStatus.next') }}</button>
          </div>
        </section>

        <p class="text-sm text-gray-500">{{ t('admin.serviceStatus.recoveryNotice') }}</p>
        <section v-for="section in incidentSections" :key="section.key" class="card p-4 sm:p-6" :data-testid="section.key">
          <h2 class="text-lg font-semibold">{{ section.title }}</h2>
          <p v-if="!section.items.length" class="mt-3 text-sm text-gray-500">{{ t('admin.serviceStatus.noIncidents') }}</p>
          <div v-for="incident in section.items" :key="incident.id" class="mt-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <div class="flex flex-wrap justify-between gap-2"><h3 class="break-all font-medium">{{ incident.platform }} · {{ incident.group_name }} (#{{ incident.group_id }}) · {{ incident.requested_model || '—' }}</h3><span class="text-sm">{{ phaseLabel(incident.phase) }}</span></div>
            <dl class="mt-3 grid gap-2 text-xs text-gray-500 sm:grid-cols-2">
              <div><dt>{{ t('admin.serviceStatus.detectedAt') }}</dt><dd>{{ timestamp(incident.detected_at) }} UTC</dd></div>
              <div><dt>{{ t('admin.serviceStatus.lastAbnormal') }}</dt><dd>{{ timestamp(incident.last_abnormal_at) }} UTC</dd></div>
              <div><dt>{{ t('admin.serviceStatus.lastEvidence') }}</dt><dd>{{ timestamp(incident.last_evidence_at) }} UTC</dd></div>
              <div v-if="incident.resolved_at"><dt>{{ t('admin.serviceStatus.resolvedAt') }}</dt><dd>{{ timestamp(incident.resolved_at) }} UTC</dd></div>
            </dl>
            <details v-if="incident.updates.length" class="mt-3 text-xs"><summary class="cursor-pointer">{{ t('admin.serviceStatus.updates') }}</summary><ol class="mt-2 space-y-1"><li v-for="(update, index) in incident.updates" :key="index">{{ timestamp(update.at) }} UTC · {{ phaseLabel(update.phase) }}</li></ol></details>
          </div>
        </section>
      </template>

      <details v-if="draft && !forbidden" class="card p-4 sm:p-6" open>
        <summary class="cursor-pointer text-lg font-semibold">{{ t('admin.serviceStatus.config') }} · v{{ draft.version }}</summary>
        <p class="mt-3 text-sm text-gray-500">{{ t('admin.serviceStatus.configNotice') }}</p>
        <form class="mt-4 space-y-5" @submit.prevent="saveConfig">
          <fieldset :disabled="saving || loadingConfig || conflict" class="space-y-5">
            <label class="flex items-center gap-3"><input v-model="draft.enabled" type="checkbox" class="h-4 w-4" data-testid="config-enabled" />{{ t('admin.serviceStatus.enabled') }}</label>
            <fieldset><legend class="input-label">{{ t('admin.serviceStatus.platforms') }}</legend><div class="flex flex-wrap gap-x-5 gap-y-3"><label v-for="name in SERVICE_STATUS_PLATFORMS" :key="name" class="flex items-center gap-2 text-sm"><input v-model="draft.platforms" type="checkbox" :value="name" class="h-4 w-4" />{{ name }}</label></div></fieldset>
            <div class="grid gap-4 md:grid-cols-2">
              <div v-for="field in numericFields" :key="field.key"><label :for="`config-${field.key}`" class="input-label">{{ t(`admin.serviceStatus.${field.label}`) }}</label><input :id="`config-${field.key}`" v-model.number="draft[field.key]" type="number" class="input" required :min="field.min" :max="field.max" :step="field.step" /></div>
            </div>
            <button type="submit" class="btn btn-primary w-full sm:w-auto" data-testid="save-config">{{ t(saving ? 'admin.serviceStatus.saving' : 'admin.serviceStatus.save') }}</button>
          </fieldset>
        </form>
      </details>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import AppLayout from '@/components/layout/AppLayout.vue'
import ServiceStatusMetrics from '@/components/service-status/ServiceStatusMetrics.vue'
import {
  SERVICE_STATUS_PLATFORMS, getConfig, getSnapshot, updateConfig,
  type ServiceStatusConfig, type ServiceStatusHistoryPoint, type ServiceStatusInterval,
  type ServiceStatusRange, type ServiceStatusScope, type ServiceStatusSnapshot
} from '@/api/admin/serviceStatus'

const { t, te } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const config = ref<ServiceStatusConfig | null>(null)
const draft = ref<ServiceStatusConfig | null>(null)
const snapshot = ref<ServiceStatusSnapshot | null>(null)
const platform = ref('')
const range = ref<ServiceStatusRange>('24h')
const ranges: ServiceStatusRange[] = ['24h', '7d', '30d']
const message = ref('')
const forbidden = ref(false)
const conflict = ref(false)
const loadingConfig = ref(false)
const loadingSnapshot = ref(false)
const saving = ref(false)
const historyIndex = ref(0)
const historyPages = computed(() => Math.ceil((snapshot.value?.history.length ?? 0) / 50))
const historyPage = computed(() => snapshot.value?.history.slice(historyIndex.value * 50, (historyIndex.value + 1) * 50) ?? [])
const canAccess = computed(() => auth.isAuthenticated && auth.isAdmin && route.path === '/admin/service-status')
const incidentSections = computed(() => [
  { key: 'open-incidents', title: t('admin.serviceStatus.openIncidents', { count: snapshot.value?.open_incident_count ?? 0 }), items: snapshot.value?.incidents.filter(item => item.phase !== 'resolved') ?? [] },
  { key: 'resolved-incidents', title: t('admin.serviceStatus.recoveredHistory'), items: snapshot.value?.incidents.filter(item => item.phase === 'resolved') ?? [] }
])
const numericFields = [
  { key: 'minimum_samples', label: 'minimumSamples', min: 1, max: 10000, step: 1 },
  { key: 'warning_error_rate', label: 'warningErrorRate', min: 0, max: 1, step: 'any' },
  { key: 'outage_error_rate', label: 'outageErrorRate', min: 0, max: 1, step: 'any' },
  { key: 'abnormal_windows', label: 'abnormalWindows', min: 1, max: 10, step: 1 },
  { key: 'recovery_windows', label: 'recoveryWindows', min: 1, max: 10, step: 1 }
] as const

// 页面代际隔离身份、路由及配置；快照代际额外隔离筛选与刷新。
let generation = 0
let snapshotGeneration = 0
let configController: AbortController | undefined
let snapshotController: AbortController | undefined
function clearSnapshot() {
  snapshotGeneration++
  snapshotController?.abort()
  snapshot.value = null
  historyIndex.value = 0
  loadingSnapshot.value = false
}
function invalidate() {
  generation++
  configController?.abort()
  clearSnapshot()
  loadingConfig.value = false
  saving.value = false
}
function revoke() {
  invalidate()
  forbidden.value = true
  config.value = null
  draft.value = null
  message.value = 'forbidden'
}
function isCurrent(epoch: number) { return epoch === generation && canAccess.value && !forbidden.value }
function errorInfo(error: unknown) { return error as { status?: number; code?: string | number; reason?: string } }
function restricted(error: unknown, epoch: number): boolean {
  if (!isCurrent(epoch)) return true
  const { status, code, reason } = errorInfo(error)
  if (status === 401 || status === 403) { revoke(); return true }
  if (status === 503 && (reason === 'SERVICE_STATUS_DISABLED' || code === 'SERVICE_STATUS_DISABLED')) {
    invalidate()
    if (config.value) config.value.enabled = false
    if (draft.value) draft.value.enabled = false
    message.value = 'disabled'
    return true
  }
  return false
}
async function loadConfig() {
  if (!canAccess.value || forbidden.value) return
  invalidate()
  const epoch = generation
  config.value = null
  draft.value = null
  message.value = ''
  conflict.value = false
  loadingConfig.value = true
  configController = new AbortController()
  try {
    const result = await getConfig(configController.signal)
    if (!isCurrent(epoch)) return
    config.value = result
    draft.value = { ...result, platforms: [...result.platforms] }
    if (platform.value && !result.platforms.includes(platform.value)) platform.value = ''
    if (!result.enabled) { clearSnapshot(); message.value = 'disabled' }
    else await loadSnapshot()
  } catch (error) {
    if (!restricted(error, epoch)) message.value = 'configFailed'
  } finally { if (isCurrent(epoch)) loadingConfig.value = false }
}
async function loadSnapshot() {
  clearSnapshot()
  if (!canAccess.value || forbidden.value || !config.value?.enabled || conflict.value || saving.value) return
  const epoch = generation
  const request = snapshotGeneration
  loadingSnapshot.value = true
  message.value = ''
  snapshotController = new AbortController()
  try {
    const result = await getSnapshot({ range: range.value, platform: platform.value || undefined }, snapshotController.signal)
    if (!isCurrent(epoch) || request !== snapshotGeneration) return
    if (!result.monitoring_enabled) {
      invalidate()
      config.value.enabled = false
      if (draft.value) draft.value.enabled = false
      message.value = 'disabled'
      return
    }
    snapshot.value = result
  } catch (error) {
    // 同一身份的拒绝或停用即使来自旧筛选，也必须撤销全部全站数据。
    if (!restricted(error, epoch) && request === snapshotGeneration) message.value = 'loadFailed'
  } finally { if (isCurrent(epoch) && request === snapshotGeneration) loadingSnapshot.value = false }
}
function validConfig(value: ServiceStatusConfig): boolean {
  return [value.minimum_samples, value.abnormal_windows, value.recovery_windows].every(Number.isInteger)
    && value.minimum_samples >= 1 && value.minimum_samples <= 10000
    && value.abnormal_windows >= 1 && value.abnormal_windows <= 10
    && value.recovery_windows >= 1 && value.recovery_windows <= 10
    && Number.isFinite(value.warning_error_rate) && value.warning_error_rate > 0 && value.warning_error_rate < 1
    && Number.isFinite(value.outage_error_rate) && value.outage_error_rate > value.warning_error_rate && value.outage_error_rate <= 1
}
async function saveConfig() {
  if (!draft.value || !canAccess.value || forbidden.value || saving.value || conflict.value) return
  if (!validConfig(draft.value)) { message.value = 'invalidConfig'; return }
  const value = { ...draft.value, platforms: [...draft.value.platforms] }
  invalidate()
  const epoch = generation
  saving.value = true
  message.value = ''
  configController = new AbortController()
  try {
    const result = await updateConfig(value, configController.signal)
    if (!isCurrent(epoch)) return
    config.value = result
    draft.value = { ...result, platforms: [...result.platforms] }
    saving.value = false
    if (platform.value && !result.platforms.includes(platform.value)) platform.value = ''
    if (!result.enabled) { message.value = 'disabled'; clearSnapshot() }
    else { await loadSnapshot(); if (isCurrent(epoch) && !message.value) message.value = 'saved' }
  } catch (error) {
    if (!restricted(error, epoch)) {
      conflict.value = errorInfo(error).status === 409
      message.value = conflict.value ? 'conflict' : 'saveFailed'
    }
  } finally { if (isCurrent(epoch)) saving.value = false }
}
function timestamp(value: string | null): string {
  if (!value) return '—'
  return new Date(value).toISOString().replace('T', ' ').replace(/\.\d{3}Z$/, '')
}
function interval(value: ServiceStatusInterval) { return `${timestamp(value.start)} → ${timestamp(value.end)}` }
function scopeKey(value: ServiceStatusScope) { return `${value.platform}:${value.group_id}:${value.requested_model}` }
function healthLabel(value: string) { return t(`admin.serviceStatus.health.${te(`admin.serviceStatus.health.${value}`) ? value : 'unknown'}`) }
function phaseLabel(value: string) { return te(`admin.serviceStatus.phases.${value}`) ? t(`admin.serviceStatus.phases.${value}`) : t('admin.serviceStatus.phases.awaiting_data') }
function reasonLabel(value?: string) { return t(`admin.serviceStatus.reasons.${value && te(`admin.serviceStatus.reasons.${value}`) ? value : 'unknown'}`) }
function healthClass(value: string) {
  if (value === 'operational') return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300'
  if (value === 'outage') return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300'
  if (value === 'degraded' || value === 'recovering') return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}
function historyUnknown(value: ServiceStatusHistoryPoint) {
  return !value.covered || value.unknown > 0 || value.success_rate === null || value.error_rate === null
}

watch([platform, range], () => { void loadSnapshot() })
watch(() => [auth.user?.id, auth.isAdmin, auth.isAuthenticated, auth.token, route.fullPath], () => {
  invalidate()
  config.value = null
  draft.value = null
  forbidden.value = false
  conflict.value = false
  message.value = ''
  if (canAccess.value) void loadConfig()
  else message.value = 'forbidden'
}, { immediate: true, flush: 'sync' })
onBeforeUnmount(() => { invalidate(); config.value = null; draft.value = null })
</script>
