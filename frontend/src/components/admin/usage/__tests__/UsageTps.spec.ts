import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/locales/en/dashboard'
import zh from '@/i18n/locales/zh/dashboard'
import UsageTps from '../UsageTps.vue'

enableAutoUnmount(afterEach)

// Vitest 使用无编译器的 i18n 运行时；这些文案没有插值，注册真实文案的消息函数。
const tpsMessages = (messages: typeof en.usage) => ({
  usage: {
    latencyTps: () => messages.latencyTps,
    averageOutputTps: () => messages.averageOutputTps,
    tpsDescription: () => messages.tpsDescription,
    tpsInvalidOutput: () => messages.tpsInvalidOutput,
    tpsInvalidDuration: () => messages.tpsInvalidDuration,
    tpsNotApplicable: () => messages.tpsNotApplicable,
    tpsMediaUnknown: () => messages.tpsMediaUnknown,
    tpsInvalidMedia: () => messages.tpsInvalidMedia,
    partialResponse: () => messages.partialResponse,
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
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('42.2 tok/s')
  })

  it('历史记录及首字耗时变化都不影响 TPS', async () => {
    const row = { ...textRow, billing_mode: undefined, request_type: undefined, first_token_ms: undefined }
    const wrapper = renderTps(row)
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('42.2 tok/s')
    await wrapper.setProps({ row: { ...row, first_token_ms: 24000 } })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('42.2 tok/s')
  })

  it.each([undefined, null, 0, -1, NaN, Infinity, -Infinity])('输出 %s 不可用并说明原因', async (output_tokens) => {
    const wrapper = renderTps({ output_tokens })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('—')
    await wrapper.get('[data-testid="usage-tps-details"]').trigger('click')
    expect(wrapper.get('[role="tooltip"]').text()).toContain(en.usage.tpsInvalidOutput)
  })

  it.each([undefined, null, 0, -1, NaN, Infinity, -Infinity])('总耗时 %s 不可用并说明原因', async (duration_ms) => {
    const wrapper = renderTps({ duration_ms })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('—')
    await wrapper.get('[data-testid="usage-tps-details"]').trigger('click')
    expect(wrapper.get('[role="tooltip"]').text()).toContain(en.usage.tpsInvalidDuration)
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
  ])('媒体或非普通生成记录 %j 不显示文本 TPS', async (row) => {
    const wrapper = renderTps(row)
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('—')
    await wrapper.get('[data-testid="usage-tps-details"]').trigger('click')
    expect(wrapper.get('[role="tooltip"]').text()).toContain(en.usage.tpsNotApplicable)
  })

  it('只有图片输入、仍输出文本时可以计算', () => {
    const wrapper = renderTps({ image_input_tokens: 500 })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('42.2 tok/s')
  })

  it.each(['en', 'zh'] as const)('%s 新记录音频未知和部分结果都提供对应说明', async (locale) => {
    const wrapper = renderTps({ timing_version: 1, audio_output_tokens: null }, locale)
    const messages = locale === 'zh' ? zh : en
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe('—')
    await wrapper.get('[data-testid="usage-tps-details"]').trigger('click')
    expect(wrapper.get('[role="tooltip"]').text()).toContain(messages.usage.tpsMediaUnknown)
    await wrapper.setProps({ row: { ...textRow, timing_version: 1, audio_output_tokens: 0, is_complete: false, completion_status: 'client_disconnected' } })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toContain('42.2 tok/s')
    expect(wrapper.get('[data-testid="usage-partial-response"]').text()).toBe(messages.usage.partialResponse)
    expect(wrapper.get('[role="tooltip"]').text()).toContain(messages.usage.partialResponse)
  })

  it.each([
    [1, 200000, '0.005 tok/s'],
    [1, 123456, '0.0081 tok/s'],
    [1, 1000000000, '0.000001 tok/s'],
    [99, 1000000, '0.099 tok/s'],
    [1, 10000, '0.1 tok/s'],
    [7, 1000, '7 tok/s'],
    [4219, 100000, '42.2 tok/s'],
    [9999, 100000, '100 tok/s'],
    [100, 1000, '100 tok/s'],
    [15045, 100000, '150 tok/s'],
    [1505, 10000, '151 tok/s'],
    [1, 100000, '0.01 tok/s'],
  ])('保留低速正值的含义：%s token / %s ms', (output_tokens, duration_ms, expected) => {
    const wrapper = renderTps({ output_tokens, duration_ms })
    expect(wrapper.get('[data-testid="usage-tps-value"]').text()).toBe(expected)
  })

  it.each(['en', 'zh'] as const)('%s 只在点击圆圈后显示 TPS 说明', async (locale) => {
    const wrapper = renderTps({ duration_ms: null }, locale)
    const messages = locale === 'zh' ? zh : en
    const label = wrapper.get('[data-testid="usage-tps-label"]')
    const value = wrapper.get('[data-testid="usage-tps-value"]')
    const trigger = wrapper.get('[data-testid="usage-tps-details"]')
    const tooltip = () => wrapper.get('[role="tooltip"]')
    expect(label.text()).toBe('TPS')
    for (const target of [label, value]) {
      expect(target.attributes('title')).toBeUndefined()
      await target.trigger('mouseenter')
      expect(tooltip().isVisible()).toBe(false)
    }
    expect(trigger.attributes('type')).toBe('button')
    await trigger.trigger('click')
    expect(tooltip().isVisible()).toBe(true)
    expect(tooltip().text()).toContain(messages.usage.tpsDescription)
    expect(tooltip().text()).toContain(messages.usage.tpsInvalidDuration)
    await trigger.trigger('click')
    expect(tooltip().isVisible()).toBe(false)
  })

  it.each([0.005, 7, 150.45])('有效 TPS %s 统一使用青色，不按速度分档', (output_tokens) => {
    const wrapper = renderTps({ output_tokens, duration_ms: 1000 })
    expect(wrapper.get('[data-testid="usage-tps-value"]').classes()).toEqual(expect.arrayContaining(['text-cyan-600', 'dark:text-cyan-400']))
  })

  it('不可用 TPS 使用灰色', () => {
    const wrapper = renderTps({ output_tokens: 0 })
    const value = wrapper.get('[data-testid="usage-tps-value"]')
    expect(value.classes()).toEqual(expect.arrayContaining(['text-gray-400', 'dark:text-gray-500']))
    expect(value.classes()).not.toContain('text-cyan-600')
  })
})
