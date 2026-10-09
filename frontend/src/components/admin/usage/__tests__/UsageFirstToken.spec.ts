import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/locales/en/dashboard'
import zh from '@/i18n/locales/zh/dashboard'
import UsageFirstToken from '../UsageFirstToken.vue'
import type { UsageTimingRow } from '@/utils/usageTiming'

enableAutoUnmount(afterEach)

const timingMessages = (messages: typeof en.usage) => ({
  usage: {
    ...Object.fromEntries([
      'firstToken', 'legacyFirstToken', 'strictFirstToken', 'strictFirstTokenDescription',
      'firstTokenDescription', 'nonStreamingTimingDescription', 'timingNotCollected',
      'lastToken', 'firstOutput', 'firstOutputKind', 'latencyDuration', 'completionStatus',
      'isComplete', 'usageSource', 'unknown',
    ].map(key => [key, () => messages[key as keyof typeof messages]])),
    outputKinds: Object.fromEntries(Object.entries(messages.outputKinds).map(([key, value]) => [key, () => value])),
    completionStatuses: Object.fromEntries(Object.entries(messages.completionStatuses).map(([key, value]) => [key, () => value])),
    usageSources: Object.fromEntries(Object.entries(messages.usageSources).map(([key, value]) => [key, () => value])),
  },
  common: { yes: () => 'Yes', no: () => 'No' },
})

const textRow: UsageTimingRow = {
  output_tokens: 500, duration_ms: 10000, first_token_ms: 9000, image_count: 0,
  image_output_tokens: 0, billing_mode: 'token', native_compaction_v2: false,
  request_type: 'stream', stream: true,
}

const renderTiming = (row: Partial<UsageTimingRow>, locale: 'en' | 'zh') => mount(UsageFirstToken, {
  attachTo: document.body,
  props: { row: { ...textRow, ...row } },
  global: {
    plugins: [createI18n({ legacy: false, locale, messages: { en: timingMessages(en.usage), zh: timingMessages(zh.usage) } })],
    stubs: { Teleport: true },
  },
})

describe('首 Token 与首次输出详情', () => {
  it.each(['en', 'zh'] as const)('%s 历史行继续旧首字，严格字段明确未采集', async (locale) => {
    const messages = locale === 'zh' ? zh.usage : en.usage
    const wrapper = renderTiming({ timing_version: 0, strict_first_token_ms: 5 }, locale)
    expect(wrapper.get('[data-testid="usage-first-token-label"]').text()).toBe(messages.legacyFirstToken)
    expect(wrapper.get('[data-testid="usage-first-token-value"]').text()).toBe('9.00s')
    const trigger = wrapper.get('[data-testid="usage-timing-details"]')
    const tooltip = () => wrapper.get('[role="tooltip"]')
    expect(tooltip().isVisible()).toBe(false)
    for (const target of wrapper.findAll('[data-testid="usage-first-token-label"], [data-testid="usage-first-token-value"]')) {
      expect(target.attributes('title')).toBeUndefined()
      await target.trigger('mouseenter')
      expect(tooltip().isVisible()).toBe(false)
    }
    await trigger.trigger('click')
    expect(tooltip().isVisible()).toBe(true)
    expect(tooltip().text()).toContain(messages.firstTokenDescription)
    expect(wrapper.findAll('dd').slice(0, 3).map(node => node.text())).toEqual(Array(3).fill(messages.timingNotCollected))
    expect(wrapper.findAll('dd').slice(-3).map(node => node.text())).toEqual(Array(3).fill(messages.unknown))
  })

  it.each(['en', 'zh'] as const)('%s 新行分开显示严格、媒体首次输出和旧口径', async (locale) => {
    const messages = locale === 'zh' ? zh.usage : en.usage
    const wrapper = renderTiming({
      timing_version: 1, strict_first_token_ms: 300, last_token_ms: 302,
      first_output_ms: 25, first_output_kind: 'image', completion_status: 'client_disconnected',
      is_complete: false, usage_source: 'upstream_partial',
    }, locale)
    expect(wrapper.get('[data-testid="usage-first-token-label"]').text()).toBe(messages.firstToken)
    expect(wrapper.get('[data-testid="usage-first-token-value"]').text()).toBe('300ms')
    await wrapper.get('[data-testid="usage-timing-details"]').trigger('click')
    expect(wrapper.findAll('dd').map(node => node.text())).toEqual([
      '300ms', '302ms', '25ms', messages.outputKinds.image, '9.00s', '10.00s',
      messages.completionStatuses.client_disconnected, 'No', messages.usageSources.upstream_partial,
    ])
    expect(wrapper.get('[role="tooltip"]').text()).toContain(messages.strictFirstTokenDescription)
    expect(wrapper.get('[role="tooltip"]').text()).not.toContain(messages.firstTokenDescription)
  })

  it.each(['sync', 'ws_v2'] as const)('%s 新记录严格时点缺失不回退旧首字，说明非流式观察边界', async (requestType) => {
    const wrapper = renderTiming({ timing_version: 1, strict_first_token_ms: null, request_type: requestType, stream: requestType === 'ws_v2' }, 'en')
    expect(wrapper.get('[data-testid="usage-first-token-value"]').text()).toBe('—')
    await wrapper.get('[data-testid="usage-timing-details"]').trigger('click')
    expect(wrapper.get('[role="tooltip"]').text()).toContain(en.usage.nonStreamingTimingDescription)
  })
})
