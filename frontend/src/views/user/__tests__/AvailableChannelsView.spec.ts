import { baseCompile } from '@intlify/message-compiler'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import AvailableChannelsView from '../AvailableChannelsView.vue'
import zh from '@/i18n/locales/zh/dashboard'
import { catalogFixture, catalogGroup, catalogOffer } from '@/__tests__/fixtures/modelCatalog'
import type { AvailableModelCatalog } from '@/api/channels'

const state = vi.hoisted(() => ({ getCatalog: vi.fn(), auth: null as { user: { id: number } | null } | null }))
vi.mock('@/api/channels', () => ({ default: { getCatalog: state.getCatalog } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => state.auth }))
const wrappers: ReturnType<typeof mount>[] = []
function render() {
  const wrapper = mount(AvailableChannelsView, { global: {
    plugins: [createI18n({ messageCompiler: message => new Function('return ' + baseCompile(String(message), { mode: 'arrow' }).code)(), legacy: false, locale: 'zh', messages: { zh: { ...zh, common: { loading: '加载中', refresh: '刷新', error: '错误' } } } })],
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
      ModelCatalogCard: { props: ['model'], template: '<div data-model>{{ model.name }}:{{ model.offers.length }}</div>' }
    }
  } })
  wrappers.push(wrapper)
  return wrapper
}
function pending() {
  let resolve!: (value: AvailableModelCatalog) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<AvailableModelCatalog>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
beforeEach(() => { state.getCatalog.mockReset(); state.auth = reactive({ user: { id: 1 } }) })
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

describe('可用渠道目录页面', () => {
  it('检索和分组过滤本地目录，空模型组保留说明', async () => {
    state.getCatalog.mockResolvedValue(catalogFixture())
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('[data-model]')).toHaveLength(3)
    expect(wrapper.text()).toContain('Empty group')
    await wrapper.get('input').setValue('gpt-catalog')
    expect(wrapper.get('[data-model]').text()).toBe('GPT-Catalog:2')
    await wrapper.get('[data-testid="catalog-group"]').setValue(2)
    expect(wrapper.get('[data-model]').text()).toBe('GPT-Catalog:1')
    expect(state.getCatalog).toHaveBeenCalledTimes(1)
  })
  it('刷新失败清除旧报价并显式重试；倍率不可用与空权限分别展示', async () => {
    state.getCatalog.mockResolvedValueOnce(catalogFixture()).mockRejectedValueOnce({ message: '授权读取失败' })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    expect(wrapper.find('[data-model]').exists()).toBe(false)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('授权读取失败')
    state.getCatalog.mockResolvedValueOnce({ ...catalogFixture(), user_rate_status: 'unavailable' })
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('个人倍率读取失败')
    state.getCatalog.mockResolvedValueOnce({ groups: [], user_rate_status: 'not_requested' })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('当前没有可查看的模型目录')
    expect(wrapper.text()).not.toContain('个人倍率读取失败')
  })
  it('用户切换取消旧请求；晚到的旧响应和旧错误不能覆盖新目录，退出及卸载清除', async () => {
    const old = pending(), next = pending()
    state.getCatalog.mockReturnValueOnce(old.promise).mockReturnValueOnce(next.promise)
    const wrapper = render()
    const firstSignal = state.getCatalog.mock.calls[0][0].signal as AbortSignal
    state.auth!.user = { id: 2 }
    expect(firstSignal.aborted).toBe(true)
    next.resolve({ user_rate_status: 'loaded', groups: [catalogGroup(2, 'New user', [catalogOffer('New-user-model', 'new')])] })
    await flushPromises()
    old.resolve(catalogFixture())
    await flushPromises()
    expect(wrapper.text()).toContain('New-user-model')
    expect(wrapper.text()).not.toContain('GPT-Catalog')
    const staleError = pending()
    state.getCatalog.mockReturnValueOnce(staleError.promise).mockResolvedValueOnce({ groups: [], user_rate_status: 'not_requested' })
    await wrapper.get('button').trigger('click')
    state.auth!.user = { id: 3 }
    await flushPromises()
    staleError.reject({ message: '旧用户错误' })
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    state.auth!.user = null
    expect(wrapper.find('[data-model]').exists()).toBe(false)
    wrapper.unmount()
    expect((state.getCatalog.mock.calls.at(-1)![0].signal as AbortSignal).aborted).toBe(true)
  })
})
