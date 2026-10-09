import { baseCompile } from '@intlify/message-compiler'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'
import ModelCatalogCard from '../ModelCatalogCard.vue'
import zh from '@/i18n/locales/zh/dashboard'
import en from '@/i18n/locales/en/dashboard'
import { catalogFixture, catalogGroup, catalogOffer } from '@/__tests__/fixtures/modelCatalog'
import { collectCatalogModels } from '@/utils/modelCatalog'

function mountCard(locale = 'zh') {
  const model = collectCatalogModels(catalogFixture()).find(card => card.name === 'GPT-Catalog')!
  return mount(ModelCatalogCard, { props: { model, userRateStatus: 'loaded' }, global: { plugins: [createI18n({ messageCompiler: message => new Function('return ' + baseCompile(String(message), { mode: 'arrow' }).code)(), legacy: false, locale, messages: { zh, en } })] } })
}

describe('模型报价详情', () => {
  it('显示整单阶梯/单位/缓存零与未知，组倍率只应用一次；切档不再乘档位倍率', async () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('USD / 1M token')
    expect(wrapper.text()).toContain('(100000, ∞]')
    expect(wrapper.findAll('[data-price="input_price"]').map(cell => cell.text())).toEqual(['$1.5', '$3'])
    expect(wrapper.get('[data-price="cache_write_price"]').text()).toBe('—')
    expect(wrapper.get('[data-price="cache_write_1h_price"]').text()).toBe('$0')
    await wrapper.get('[data-testid="catalog-tier"]').setValue('priority')
    expect(wrapper.get('[data-price="input_price"]').text()).toBe('$3')
    expect(wrapper.text()).toContain('high ×2')
    expect(wrapper.text()).toContain('10:00–12:00 ×1.5')
  })
  it('报价切换保留个人零倍率；英文标签完整', async () => {
    const wrapper = mountCard('en')
    await wrapper.get('[data-testid="catalog-offer"]').setValue('exclusive-offer')
    expect(wrapper.get('[data-testid="catalog-rate"]').text()).toBe('×0')
    expect(wrapper.get('[data-price="input_price"]').text()).toBe('$0')
    expect(wrapper.get('[data-price="cache_write_price"]').text()).toBe('—')
    expect(wrapper.text()).toContain('Personal override')
    expect(wrapper.text()).not.toContain('availableChannels.')
  })
  it('未知单位/实际请求依赖报价显示原因，未擅自显示每次费用', async () => {
    const wrapper = mountCard()
    const offer = { ...catalogOffer(), billing_mode: 'per_request', billing_unit: 'unknown', price_status: 'unknown' as const, price_reason: 'unsupported_unit' as const, pricing: null }
    await wrapper.setProps({ model: collectCatalogModels({ user_rate_status: 'loaded', groups: [catalogGroup(1, 'Media', [offer])] })[0] })
    expect(wrapper.text()).toContain('单位未确认')
    expect(wrapper.text()).toContain('当前目录尚未支持此计费单位')
    expect(wrapper.find('table').exists()).toBe(false)
    await wrapper.setProps({ model: collectCatalogModels({ user_rate_status: 'loaded', groups: [catalogGroup(1, 'Dependent', [{ ...offer, price_reason: 'request_dependent' }])] })[0] })
    expect(wrapper.text()).toContain('价格依赖实际路由或响应模型')
  })
})
