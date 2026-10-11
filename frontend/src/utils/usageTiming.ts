import type { UsageCompletionStatus, UsageLog } from '@/types'
import { BILLING_MODE_TOKEN, getDisplayBillingMode } from './billingMode'
import { isUsageRequestType, resolveUsageRequestType } from './usageRequestType'

export type UsageTimingRow = Pick<UsageLog,
  'output_tokens' | 'duration_ms' | 'first_token_ms' | 'billing_mode' | 'image_count' |
  'image_output_tokens' | 'native_compaction_v2' | 'request_type' | 'stream' | 'openai_ws_mode' |
  'timing_version' | 'strict_first_token_ms' | 'last_token_ms' | 'first_output_ms' |
  'first_output_kind' | 'audio_output_tokens' | 'completion_status' | 'is_complete' | 'usage_source'
>

type Translate = (key: string) => string
export interface UsageAverageTps {
  value: number | null
  reason: string | null
  partial: boolean
}

const validNonnegative = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value) && value >= 0

export const hasStrictUsageTiming = (row: UsageTimingRow): boolean => row.timing_version === 1

export const strictFirstTokenMs = (row: UsageTimingRow): number | null =>
  hasStrictUsageTiming(row) && validNonnegative(row.strict_first_token_ms)
    ? row.strict_first_token_ms : null

export const displayedFirstTokenMs = (row: UsageTimingRow): number | null =>
  hasStrictUsageTiming(row)
    ? strictFirstTokenMs(row)
    : validNonnegative(row.first_token_ms) ? row.first_token_ms : null

const completionStatuses: UsageCompletionStatus[] = [
  'unknown', 'completed', 'client_disconnected', 'upstream_error', 'interrupted',
]

export const usageCompletionStatus = (row: UsageTimingRow): UsageCompletionStatus =>
  row.completion_status && completionStatuses.includes(row.completion_status)
    ? row.completion_status : 'unknown'

export const isPartialUsage = (row: UsageTimingRow): boolean => {
  const status = usageCompletionStatus(row)
  return row.is_complete === false || row.usage_source === 'upstream_partial' ||
    status === 'client_disconnected' || status === 'upstream_error' || status === 'interrupted'
}

// 单一计算入口供表格和两种导出复用。首/末 Token 不能缩短总耗时分母。
export const averageUsageTps = (row: UsageTimingRow): UsageAverageTps => {
  const partial = isPartialUsage(row)
  const unavailable = (reason: string): UsageAverageTps => ({ value: null, reason, partial })
  const requestType = resolveUsageRequestType(row)
  const mode = getDisplayBillingMode(row)
  if ((mode && mode !== BILLING_MODE_TOKEN) ||
      ['live', 'probe', 'gwpool_degraded'].includes(requestType) ||
      (row.request_type && !isUsageRequestType(row.request_type))) {
    return unavailable('usage.tpsNotApplicable')
  }
  if (!validNonnegative(row.output_tokens) || row.output_tokens === 0) {
    return unavailable('usage.tpsInvalidOutput')
  }
  if (!validNonnegative(row.duration_ms) || row.duration_ms === 0) {
    return unavailable('usage.tpsInvalidDuration')
  }
  // 历史普通文本沿用旧范围；旧媒体记录没有可信拆分，不能把它当成纯文本。
  const historical = row.timing_version == null || row.timing_version === 0
  if (historical && (row.image_count > 0 || (row.image_output_tokens ?? 0) > 0 ||
      (row.audio_output_tokens ?? 0) > 0 || ['image', 'audio'].includes(row.first_output_kind ?? ''))) {
    return unavailable('usage.tpsNotApplicable')
  }
  const imageTokens = historical && row.image_output_tokens == null ? 0 : row.image_output_tokens
  const audioTokens = historical && row.audio_output_tokens == null ? 0 : row.audio_output_tokens
  if (imageTokens == null || audioTokens == null || (row.image_count > 0 && imageTokens === 0)) {
    return unavailable('usage.tpsMediaUnknown')
  }
  if (!validNonnegative(imageTokens) || !validNonnegative(audioTokens)) {
    return unavailable('usage.tpsInvalidMedia')
  }
  const textTokens = row.output_tokens - imageTokens - audioTokens
  if (textTokens < 0) return unavailable('usage.tpsInvalidMedia')
  if (textTokens === 0 ||
      ((row.native_compaction_v2 || row.first_output_kind === 'compaction') && strictFirstTokenMs(row) == null)) {
    return unavailable('usage.tpsNotApplicable')
  }
  const value = textTokens * 1000 / row.duration_ms
  if (!Number.isFinite(value) || value <= 0) return unavailable('usage.tpsInvalidOutput')
  return { value, reason: null, partial }
}

export const usageTpsNote = (result: UsageAverageTps, t: Translate): string =>
  [result.reason ? t(result.reason) : '', result.partial ? t('usage.partialResponse') : '']
    .filter(Boolean).join(' ')

export const formatUsageTps = (value: number | null): string => {
  if (value == null) return '—'
  const number = value < 0.1
    ? Number(value.toPrecision(2)).toString()
    : value >= 100
      ? String(Math.round(value))
      : (Math.round(value * 10) / 10).toFixed(1).replace(/\.0$/, '')
  return `${number} tok/s`
}

export const formatUsageTimingDuration = (ms: number | null | undefined): string => {
  if (!validNonnegative(ms)) return '—'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`
  const totalSec = Math.round(ms / 1000)
  if (totalSec < 3600) return `${Math.floor(totalSec / 60)}m ${totalSec % 60}s`
  return `${Math.floor(totalSec / 3600)}h ${Math.floor((totalSec % 3600) / 60)}m`
}

const exportHeaderKeys = [
  'averageTpsExport', 'averageTpsNote', 'timingVersion', 'strictFirstTokenMs', 'lastTokenMs',
  'firstOutputMs', 'firstOutputKind', 'imageOutputTokens', 'audioOutputTokens',
  'completionStatus', 'isComplete', 'usageSource',
] as const

export const usageTimingExportHeaders = (t: Translate): string[] =>
  exportHeaderKeys.map(key => t(`usage.${key}`))

// 保留原始毫秒、枚举、布尔和空值；导出不使用 UI 取整值，也不伪造历史 0/成功。
export const usageTimingExportValues = (row: UsageTimingRow, t: Translate): Array<string | number | boolean> => {
  const result = averageUsageTps(row)
  return [
    result.value ?? '', usageTpsNote(result, t), row.timing_version ?? '',
    row.strict_first_token_ms ?? '', row.last_token_ms ?? '', row.first_output_ms ?? '',
    row.first_output_kind ?? '', row.image_output_tokens ?? '', row.audio_output_tokens ?? '',
    row.completion_status ?? 'unknown', row.is_complete ?? '', row.usage_source ?? 'unknown',
  ]
}
