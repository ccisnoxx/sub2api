import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/locales/en/dashboard'
import zh from '@/i18n/locales/zh/dashboard'
import UsageTps from '../UsageTps.vue'

enableAutoUnmount(afterEach)

// Vitest 使用无编译器的 i18n 运行时；这些文案没有插值，注册真实文案的消息函数。
const tpsMessages = (messages: typeof en.usage) => ({
  usage: {
    averageOutputTps: () => messages.averageOutputTps,
    tpsDescription: () => messages.tpsDescription,
    tpsInvalidOutput: () => messages.tpsInvalidOutput,
    tpsInvalidDuration: () => messages.tpsInvalidDuration,
    tpsNotApplicable: () => messages.tpsNotApplicable,
  },
})

const textRow = {
  output_tokens: 1040,
  duration_ms: 24650,
  first_token_ms: 1500,
  billing_mode: 'token',
  image_count: 0,
  image_output_tokens: 0,
  native_compaction_v2: false,
  request_type: 'stream' as const,
  stream: true,
}

const renderTps = (row = {}, locale: 'en' | 'zh' = 'en') => mount(UsageTps, {
  props: { row: { ...textRow, ...row } },
  global: {
    plugins: [createI18n({ legacy: false, locale, messages: { en: tpsMessages(en.usage), zh: tpsMessages(zh.usage) } })],
    stubs: { Teleport: true },
  },
})

describe('UsageTps', () => {
  it.each(['sync', 'stream', 'ws_v2'] as const)('按总耗时计算 %s 文本输出速率', (request_type) => {
    const wrapper = renderTps({ request_type })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('42.19 tok/s')
  })

  it('历史记录及首字耗时变化都不影响 TPS', async () => {
    const row = { ...textRow, billing_mode: undefined, request_type: undefined, first_token_ms: undefined }
    const wrapper = renderTps(row)
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('42.19 tok/s')
    await wrapper.setProps({ row: { ...row, first_token_ms: 24000 } })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('42.19 tok/s')
  })

  it.each([undefined, null, 0, -1, NaN, Infinity, -Infinity])('输出 %s 不可用并说明原因', (output_tokens) => {
    const wrapper = renderTps({ output_tokens })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('—')
    expect(wrapper.get('button').attributes('aria-label')).toContain(en.usage.tpsInvalidOutput)
  })

  it.each([undefined, null, 0, -1, NaN, Infinity, -Infinity])('总耗时 %s 不可用并说明原因', (duration_ms) => {
    const wrapper = renderTps({ duration_ms })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('—')
    expect(wrapper.get('button').attributes('aria-label')).toContain(en.usage.tpsInvalidDuration)
  })

  it.each([
    { billing_mode: 'image' },
    { billing_mode: 'video' },
    { billing_mode: 'per_request' },
    { billing_mode: 'audio' },
    { billing_mode: 'future-mode' },
    { billing_mode: null, image_count: 1 },
    { billing_mode: 'token', image_count: 1 },
    { billing_mode: 'token', image_output_tokens: 200 },
    { request_type: 'audio' },
    { request_type: 'video' },
    { native_compaction_v2: true },
    { request_type: 'live' },
    { request_type: 'probe' },
    { request_type: 'gwpool_degraded' },
  ])('媒体或非普通生成记录 %j 不显示文本 TPS', (row) => {
    const wrapper = renderTps(row)
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('—')
    expect(wrapper.get('button').attributes('aria-label')).toContain(en.usage.tpsNotApplicable)
  })

  it('只有图片输入、仍输出文本时可以计算', () => {
    const wrapper = renderTps({ image_input_tokens: 500 })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('42.19 tok/s')
  })

  it.each([
    [1, 200000, '<0.01 tok/s'],
    [1, 100000, '0.01 tok/s'],
  ])('保留低速正值的含义：%s token / %s ms', (output_tokens, duration_ms, expected) => {
    const wrapper = renderTps({ output_tokens, duration_ms })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe(expected)
  })

  it.each(['en', 'zh'] as const)('%s 提示可通过原生按钮打开，并解释统计口径与不可用原因', async (locale) => {
    const wrapper = renderTps({ duration_ms: null }, locale)
    const messages = locale === 'zh' ? zh : en
    const button = wrapper.get('button')
    expect(button.attributes('type')).toBe('button')
    expect(button.attributes('aria-label')).toContain(messages.usage.tpsDescription)
    expect(wrapper.get('[role="tooltip"]').isVisible()).toBe(false)
    await button.trigger('click')
    await nextTick()
    const tooltip = wrapper.get('[role="tooltip"]')
    expect(tooltip.isVisible()).toBe(true)
    expect(tooltip.text()).toContain(messages.usage.tpsInvalidDuration)
    expect(tooltip.text()).toContain(messages.usage.tpsDescription)
    await tooltip.get('button').trigger('click')
    expect(wrapper.get('[role="tooltip"]').isVisible()).toBe(false)
  })
})
