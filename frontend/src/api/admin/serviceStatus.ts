import { apiClient } from '../client'

// 与后端具体平台目录一致；复合分组不是实际平台。
export const SERVICE_STATUS_PLATFORMS = [
  'openai', 'anthropic', 'gemini', 'antigravity', 'grok',
  'kimi', 'zhipu', 'deepseek', 'minimax', 'typesafe', 'opencode_go'
] as const
export type ServiceStatusRange = '24h' | '7d' | '30d'

export interface ServiceStatusConfig {
  version: number
  enabled: boolean
  platforms: string[]
  minimum_samples: number
  warning_error_rate: number
  outage_error_rate: number
  abnormal_windows: number
  recovery_windows: number
}
export interface ServiceStatusCounts {
  success: number
  failure: number
  excluded: number
  unknown: number
}
export interface ServiceStatusMetrics extends ServiceStatusCounts {
  qualified: number
  error_rate: number | null
  success_rate: number | null
}
export interface ServiceStatusScope {
  platform: string
  group_id: number
  group_name: string
  requested_model: string
}
export interface ServiceStatusLeaf extends ServiceStatusScope, ServiceStatusMetrics {
  health: string
  unknown_reason?: string
  last_evidence_at: string | null
  incident_id: string | null
  incident_phase: string | null
}
export interface ServiceStatusInterval { start: string; end: string }
export interface ServiceStatusCoverage extends ServiceStatusInterval { platform: string }
export interface ServiceStatusHistoryPoint extends ServiceStatusScope, ServiceStatusMetrics, ServiceStatusInterval {
  covered: boolean
  unknown_reason?: string
}
export interface ServiceStatusIncidentUpdate { phase: string; at: string }
export interface ServiceStatusIncident extends ServiceStatusScope {
  id: string
  phase: string
  detected_at: string
  last_evidence_at: string
  last_abnormal_at: string
  resolved_at: string | null
  updates: ServiceStatusIncidentUpdate[]
}
export interface ServiceStatusPlatform extends ServiceStatusMetrics {
  platform: string
  health: string
  unknown_reason?: string
}
export interface ServiceStatusSnapshot {
  schema_version: number
  config_version: number
  generated_at: string
  observed_through: string | null
  window: ServiceStatusInterval
  history_range: ServiceStatusInterval
  monitoring_enabled: boolean
  enabled_platforms: string[]
  health: string
  unknown_reason?: string
  coverage: ServiceStatusCoverage[]
  gaps: string[]
  platforms: ServiceStatusPlatform[]
  scopes: ServiceStatusLeaf[]
  history: ServiceStatusHistoryPoint[]
  incidents: ServiceStatusIncident[]
  open_incident_count: number
  bounded_observation_notice: string
}

export async function getConfig(signal?: AbortSignal): Promise<ServiceStatusConfig> {
  const { data } = await apiClient.get<ServiceStatusConfig>('/admin/service-status/config', { signal })
  return data
}
export async function updateConfig(config: ServiceStatusConfig, signal?: AbortSignal): Promise<ServiceStatusConfig> {
  const { data } = await apiClient.put<ServiceStatusConfig>('/admin/service-status/config', config, { signal })
  return data
}
export async function getSnapshot(
  params: { range?: ServiceStatusRange; platform?: string } = {},
  signal?: AbortSignal
): Promise<ServiceStatusSnapshot> {
  const { data } = await apiClient.get<ServiceStatusSnapshot>('/admin/service-status/snapshot', {
    params: { range: params.range ?? '24h', ...(params.platform ? { platform: params.platform } : {}) },
    signal
  })
  return data
}

export default { getConfig, updateConfig, getSnapshot }
