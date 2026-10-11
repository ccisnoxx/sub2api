import { baseCompile } from '@intlify/message-compiler'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/locales/en/admin/ops'
import zh from '@/i18n/locales/zh/admin/ops'
import type { OpsRoutingDiagnostics } from '@/api/admin/ops'
import Panel from '../OpsRoutingDiagnosticsPanel.vue'

const snapshot = (overrides: Partial<OpsRoutingDiagnostics> = {}): OpsRoutingDiagnostics => ({
  schema_version: 1, turn: null, selection_attempt: 1, selection_layer: 'load_balance',
  selection_reason: 'pool_empty', candidate_pool: 0, filtered_candidates: 0,
  filter_reasons: {}, filter_coverage: 'complete', ...overrides
})
function render(diagnostics?: OpsRoutingDiagnostics | null, locale = 'zh') {
  return mount(Panel, { props: { diagnostics }, global: { plugins: [createI18n({
    messageCompiler: message => new Function('return ' + baseCompile(String(message), { mode: 'arrow' }).code)(),
    legacy: false, locale, messages: { en: { admin: en }, zh: { admin: zh } }
  })] } })
}
for (const locale of ['zh', 'en']) {
  describe(`路由诊断 ${locale}`, () => {
    it('retains observed zero and distinguishes an empty reason map', () => {
      const wrapper = render(snapshot(), locale)
      expect(wrapper.get('[data-routing-field="candidatePool"]').text()).toBe('0')
      expect(wrapper.get('[data-routing-field="filteredCandidates"]').text()).toBe('0')
      expect(wrapper.text()).toContain(locale === 'zh' ? '本次观察未记录过滤' : 'No filters were recorded')
      expect(wrapper.find('[data-routing-field="turn"]').exists()).toBe(false)
    })
    it('shows unknown counts and reasons before pool observation', () => {
      const wrapper = render(snapshot({ candidate_pool: null, filtered_candidates: null, filter_reasons: null,
        filter_coverage: 'unobserved', selection_layer: 'channel_pricing', selection_reason: 'channel_pricing_restricted' }), locale)
      expect(wrapper.get('[data-routing-field="candidatePool"]').text()).toBe(locale === 'zh' ? '未知' : 'Unknown')
      expect(wrapper.get('[data-routing-field="filteredCandidates"]').text()).toBe(locale === 'zh' ? '未知' : 'Unknown')
      expect(wrapper.text()).toContain(locale === 'zh' ? '未观察过滤原因' : 'Filter reasons were not observed')
      expect(wrapper.text()).toContain(locale === 'zh' ? '渠道定价限制' : 'Channel pricing restriction')
    })
    it('shows partial filtering as a lower bound with request-turn attribution', () => {
      const wrapper = render(snapshot({ turn: 2, selection_attempt: 3, candidate_pool: 5, filtered_candidates: 2,
        filter_reasons: { quota_auto_pause_5h: 1, excluded: 1 }, filter_coverage: 'partial', selection_reason: 'selection_exhausted' }), locale)
      expect(wrapper.get('[data-routing-field="candidatePool"]').text()).toBe('5')
      expect(wrapper.get('[data-routing-field="filteredCandidates"]').text()).toBe('2')
      expect(wrapper.get('[data-routing-field="turn"]').text()).toBe('2')
      expect(wrapper.get('[data-routing-field="selectionAttempt"]').text()).toBe('3')
      expect(wrapper.text()).toContain(locale === 'zh' ? '已知下界' : 'known lower bound')
      expect(wrapper.text()).toContain(locale === 'zh' ? '5小时配额暂停' : '5-hour quota pause')
      expect(wrapper.text()).not.toContain('quota_auto_pause_5h')
    })
    it.each([undefined, null])('does not treat a missing snapshot as an empty pool', value => {
      const wrapper = render(value, locale)
      expect(wrapper.text()).toContain(locale === 'zh' ? '不代表候选池为空' : 'do not mean the candidate pool was empty')
      expect(wrapper.find('[data-routing-field="candidatePool"]').exists()).toBe(false)
    })
  })
}
it('does not interpret a future schema or render unexpected snapshot data', () => {
  const wrapper = render({ ...snapshot(), schema_version: 2, secret: 'raw-auth-sentinel' } as OpsRoutingDiagnostics)
  expect(wrapper.text()).toContain('无法解释此版本')
  expect(wrapper.text()).not.toContain('raw-auth-sentinel')
  expect(wrapper.find('[data-routing-field="candidatePool"]').exists()).toBe(false)
})
it('renders only known labels instead of echoing arbitrary code text', () => {
  const wrapper = render(snapshot({ selection_layer: 'sensitive-code-sentinel', selection_reason: 'sensitive-code-sentinel',
    filter_reasons: { excluded: 1, 'sensitive-code-sentinel': 1 }, filtered_candidates: 2 }))
  expect(wrapper.get('[data-routing-field="selectionLayer"]').text()).toBe('未知')
  expect(wrapper.text()).not.toContain('sensitive-code-sentinel')
  expect(wrapper.text()).toContain('本次重选排除')
})
