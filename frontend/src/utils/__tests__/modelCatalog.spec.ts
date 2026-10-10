import { describe, expect, it } from 'vitest'
import { collectCatalogModels, filterCatalogModels, formatCatalogPrice } from '../modelCatalog'
import { catalogFixture, catalogGroup, catalogOffer } from '@/__tests__/fixtures/modelCatalog'

describe('模型目录展示合同', () => {
  it('同平台大小写变体合并但保留每份分组报价，平台相同名称不串组', () => {
    const catalog = catalogFixture()
    catalog.groups.push(catalogGroup(4, 'Other platform', [{ ...catalogOffer('GPT-Catalog', 'anthropic-offer'), platform: 'anthropic' }]))
    const cards = collectCatalogModels(catalog)
    const openai = cards.find(card => card.platform === 'openai' && card.name === 'GPT-Catalog')!
    expect(openai.offers.map(item => item.group.id)).toEqual([1, 2])
    expect(cards.filter(card => card.name.toLowerCase() === 'gpt-catalog')).toHaveLength(2)
  })
  it('按分组和报价来源检索只保留匹配报价，模型检索保留该模型各分组报价', () => {
    const cards = collectCatalogModels(catalogFixture())
    expect(filterCatalogModels(cards, ' EXCLUSIVE ', null)[0].offers.map(item => item.group.id)).toEqual([2])
    expect(filterCatalogModels(cards, 'gpt-catalog', null)[0].offers).toHaveLength(2)
    expect(filterCatalogModels(cards, 'catalog channel', 2)[0].offers).toHaveLength(1)
    expect(filterCatalogModels(cards, '', 3)).toEqual([])
  })
  it('倍率应用一次，明确零与未知独立，未知单位不猜测', () => {
    expect(formatCatalogPrice(0.000002, 0.75, 'USD/token')).toBe('$1.5')
    expect(formatCatalogPrice(0.000002, 0, 'USD/token')).toBe('$0')
    expect(formatCatalogPrice(null, 0, 'USD/token')).toBe('—')
    expect(formatCatalogPrice(0, 1, 'USD/request')).toBe('$0')
    expect(formatCatalogPrice(0.3, 2, 'USD/request')).toBe('$0.6')
    expect(formatCatalogPrice(0.3, 1, 'unknown')).toBe('—')
  })
})
