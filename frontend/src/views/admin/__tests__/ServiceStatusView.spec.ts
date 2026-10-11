import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import { baseCompile } from '@intlify/message-compiler'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ServiceStatusView from '../ServiceStatusView.vue'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import type { ServiceStatusConfig, ServiceStatusSnapshot } from '@/api/admin/serviceStatus'

const api = vi.hoisted(() => ({ getConfig: vi.fn(), getSnapshot: vi.fn(), updateConfig: vi.fn() }))
const state = reactive({ user: { id: 1 }, isAdmin: true, isAuthenticated: true, token: 'admin-one' })
const route = reactive({ path: '/admin/service-status', fullPath: '/admin/service-status' })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => state }))
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('@/api/admin/serviceStatus', async () => ({
  ...await vi.importActual<typeof import('@/api/admin/serviceStatus')>('@/api/admin/serviceStatus'), ...api
}))

const config: ServiceStatusConfig = {
  version: 4, enabled: true, platforms: ['openai', 'grok'], minimum_samples: 5,
  warning_error_rate: 0.05, outage_error_rate: 0.9, abnormal_windows: 2, recovery_windows: 3
}
const time = '2026-10-09T12:00:00Z'
function snapshot(model = 'visible-model'): ServiceStatusSnapshot {
  const metrics = { success: 5, failure: 0, excluded: 1, unknown: 0, qualified: 5, error_rate: 0, success_rate: 1 }
  const scope = { platform: 'openai', group_id: 2, group_name: 'Active group', requested_model: model }
  return {
    schema_version: 1, config_version: 4, generated_at: time, observed_through: time,
    window: { start: '2026-10-09T11:55:00Z', end: time }, history_range: { start: '2026-10-08T12:00:00Z', end: time },
    monitoring_enabled: true, enabled_platforms: ['openai', 'grok'], health: 'degraded', coverage: [], gaps: [],
    platforms: [{ platform: 'openai', health: 'operational', ...metrics }],
    scopes: [{ ...scope, ...metrics, health: 'operational', last_evidence_at: time, incident_id: null, incident_phase: null }],
    history: [{ ...scope, ...metrics, start: '2026-10-09T11:59:00Z', end: time, covered: false, success_rate: null, error_rate: null, unknown_reason: 'coverage_gap' }],
    incidents: [{ ...scope, id: 'anonymous-incident', phase: 'recovering', detected_at: time, last_abnormal_at: time, last_evidence_at: time, resolved_at: null, updates: [{ phase: 'recovering', at: time }] }],
    open_incident_count: 1, bounded_observation_notice: 'bounded_source_logs'
  }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
let wrapper: ReturnType<typeof mount>
function open(locale = 'zh') {
  wrapper = mount(ServiceStatusView, {
    global: { plugins: [createI18n({
      legacy: false, locale, messages: { zh, en },
      messageCompiler: message => new Function('return ' + baseCompile(String(message), { mode: 'arrow' }).code)()
    })], stubs: { AppLayout: { template: '<div><slot /></div>' } } }
  })
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks()
  Object.assign(state, { user: { id: 1 }, isAdmin: true, isAuthenticated: true, token: 'admin-one' })
  Object.assign(route, { path: '/admin/service-status', fullPath: '/admin/service-status' })
  api.getConfig.mockResolvedValue({ ...config, platforms: [...config.platforms] })
  api.getSnapshot.mockResolvedValue(snapshot())
  api.updateConfig.mockResolvedValue({ ...config, version: 5 })
})
afterEach(() => { wrapper?.unmount() })

describe('管理员服务状态的可见合同', () => {
  it('显示后端摘要而非加权比例，缺口灰显且不把恢复观察放入已恢复历史', async () => {
    open(); await flushPromises()
    expect(wrapper.get('[data-testid="status-summary"]').text()).toContain('服务异常')
    expect(wrapper.get('[data-testid="status-history"]').text()).toContain('—（无法判定）')
    expect(wrapper.get('[data-testid="status-history"] li').classes()).toContain('bg-gray-100')
    expect(wrapper.get('[data-testid="open-incidents"]').text()).toContain('恢复观察')
    expect(wrapper.get('[data-testid="resolved-incidents"]').text()).not.toContain('visible-model')
    expect(wrapper.text()).toContain('用户侧 / 不适用排除')
    expect(wrapper.find('a[href*="request"]').exists()).toBe(false)
  })

  it('筛选竞争取消旧请求，晚到的旧成功不能覆盖新范围', async () => {
    const old = deferred<ServiceStatusSnapshot>()
    api.getSnapshot.mockReturnValueOnce(old.promise).mockResolvedValueOnce(snapshot('new-filter-model'))
    open(); await flushPromises()
    const oldSignal = api.getSnapshot.mock.calls[0][1] as AbortSignal
    await wrapper.get('#status-platform').setValue('grok'); await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    expect(wrapper.text()).toContain('new-filter-model')
    old.resolve(snapshot('old-filter-model')); await flushPromises()
    expect(wrapper.text()).not.toContain('old-filter-model')
    expect(wrapper.text()).toContain('new-filter-model')
  })

  it('同身份旧筛选的 403 撤销新快照，其他晚到响应不能重新显示全站数据', async () => {
    const old = deferred<ServiceStatusSnapshot>()
    const later = deferred<ServiceStatusSnapshot>()
    api.getSnapshot.mockReturnValueOnce(old.promise).mockResolvedValueOnce(snapshot('new-visible')).mockReturnValueOnce(later.promise)
    open(); await flushPromises()
    await wrapper.get('#status-platform').setValue('grok'); await flushPromises()
    expect(wrapper.text()).toContain('new-visible')
    await wrapper.get('#status-range').setValue('7d'); await flushPromises()
    old.reject({ status: 403 }); await flushPromises()
    expect(wrapper.text()).toContain('已清空全站数据')
    later.resolve(snapshot('forbidden-late-model')); await flushPromises()
    expect(wrapper.find('[data-testid="status-summary"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('forbidden-late-model')
    expect(wrapper.find('[data-testid="save-config"]').exists()).toBe(false)
  })

  it('身份切换清除旧数据，并隔离旧身份晚到的拒绝', async () => {
    const old = deferred<ServiceStatusSnapshot>()
    api.getSnapshot.mockReturnValueOnce(old.promise).mockResolvedValueOnce(snapshot('second-admin-model'))
    open(); await flushPromises()
    state.user.id = 3
    await flushPromises()
    expect(wrapper.text()).toContain('second-admin-model')
    old.reject({ status: 403 }); await flushPromises()
    expect(wrapper.text()).toContain('second-admin-model')
    state.isAdmin = false
    await flushPromises()
    expect(wrapper.find('[data-testid="status-summary"]').exists()).toBe(false)
  })

  it('路由切换取消请求，晚到成功不恢复内容', async () => {
    const pending = deferred<ServiceStatusSnapshot>()
    api.getSnapshot.mockReturnValueOnce(pending.promise)
    open(); await flushPromises()
    const signal = api.getSnapshot.mock.calls[0][1] as AbortSignal
    route.path = '/admin/dashboard'; route.fullPath = '/admin/dashboard'
    await flushPromises()
    expect(signal.aborted).toBe(true)
    pending.resolve(snapshot('departed-model')); await flushPromises()
    expect(wrapper.text()).not.toContain('departed-model')
  })

  it('disabled 配置保持可编辑，不获取快照；保存发送完整版本', async () => {
    api.getConfig.mockResolvedValue({ ...config, enabled: false })
    open(); await flushPromises()
    expect(api.getSnapshot).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('独立服务状态监控已停用')
    await wrapper.get('[data-testid="config-enabled"]').setValue(true)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.updateConfig.mock.calls[0][0]).toEqual(config)
    expect(wrapper.text()).toContain('visible-model')
    expect(wrapper.text()).toContain('v5')
  })

  it('503 停用清空已显示快照，晚到成功也不可恢复', async () => {
    const old = deferred<ServiceStatusSnapshot>()
    api.getSnapshot.mockReturnValueOnce(old.promise).mockResolvedValueOnce(snapshot('before-disable')).mockRejectedValueOnce({ status: 503, code: 503, reason: 'SERVICE_STATUS_DISABLED' })
    open(); await flushPromises()
    await wrapper.get('#status-platform').setValue('grok'); await flushPromises()
    expect(wrapper.text()).toContain('before-disable')
    await wrapper.get('#status-range').setValue('7d'); await flushPromises()
    expect(wrapper.text()).toContain('独立服务状态监控已停用')
    old.resolve(snapshot('after-disable-late')); await flushPromises()
    expect(wrapper.text()).not.toContain('after-disable-late')
    expect(wrapper.find('[data-testid="status-summary"]').exists()).toBe(false)
  })

  it('409 保留未保存草稿并要求重新读取，避免重复旧版本提交', async () => {
    api.updateConfig.mockRejectedValueOnce({ status: 409 })
    open('en'); await flushPromises()
    await wrapper.get('#config-minimum_samples').setValue(10)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.text()).toContain('Another administrator changed the configuration (409)')
    expect((wrapper.get('#config-minimum_samples').element as HTMLInputElement).value).toBe('10')
    expect(wrapper.find('[data-testid="status-summary"]').exists()).toBe(false)
    await wrapper.get('form').trigger('submit')
    expect(api.updateConfig).toHaveBeenCalledTimes(1)
    api.getConfig.mockResolvedValueOnce({ ...config, version: 6 })
    await wrapper.get('header button').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('v6')
  })

  it('无平台为空范围、低样本 NULL 及未知终态均保留明确解释', async () => {
    const empty = snapshot()
    empty.enabled_platforms = []; empty.scopes = []; empty.platforms = []
    empty.health = 'unknown'; empty.unknown_reason = 'no_enabled_platforms'
    api.getSnapshot.mockResolvedValueOnce(empty)
    open(); await flushPromises()
    expect(wrapper.text()).toContain('暂无启用的监控平台')
    const low = snapshot()
    low.scopes[0] = { ...low.scopes[0]!, health: 'unknown', unknown: 1, unknown_reason: 'terminal_unknown', success_rate: null, error_rate: null }
    low.history[0] = { ...low.history[0]!, covered: true, unknown_reason: 'insufficient_samples', qualified: 2, success_rate: null, error_rate: null }
    api.getSnapshot.mockResolvedValueOnce(low)
    await wrapper.get('#status-range').setValue('30d'); await flushPromises()
    expect(wrapper.text()).toContain('包含未知终态')
    expect(wrapper.get('[data-testid="status-history"]').text()).toContain('合格请求样本不足')
  })
})
