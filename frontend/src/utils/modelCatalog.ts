import type { AvailableModelCatalog, CatalogGroup, CatalogOffer } from '@/api/channels'
import { formatScaled } from '@/utils/pricing'

export interface ModelCatalogCard {
  key: string
  name: string
  platform: string
  offers: { group: CatalogGroup; offer: CatalogOffer }[]
}

/** 只聚合已授权目录；报价保持分组顺序，不按最低价选择或折叠。 */
export function collectCatalogModels(catalog: AvailableModelCatalog): ModelCatalogCard[] {
  const cards = new Map<string, ModelCatalogCard>()
  for (const group of catalog.groups) {
    for (const offer of group.models) {
      const key = JSON.stringify([offer.platform, offer.name.toLowerCase()])
      let card = cards.get(key)
      if (!card) {
        card = { key, name: offer.name, platform: offer.platform, offers: [] }
        cards.set(key, card)
      }
      if (!card.offers.some((entry) => entry.offer.offer_key === offer.offer_key)) {
        card.offers.push({ group, offer })
      }
    }
  }
  return [...cards.values()].sort((a, b) => a.key < b.key ? -1 : a.key > b.key ? 1 : 0)
}

export function filterCatalogModels(cards: ModelCatalogCard[], query: string, groupId: number | null): ModelCatalogCard[] {
  const q = query.trim().toLowerCase()
  return cards.flatMap((card) => {
    const eligible = card.offers.filter(({ group }) => groupId === null || group.id === groupId)
    const modelHit = !q || [card.name, card.platform].some((value) => value.toLowerCase().includes(q))
    const offers = modelHit ? eligible : eligible.filter(({ group, offer }) =>
      [group.name, group.description, offer.source.name, offer.source.description]
        .some((value) => value.toLowerCase().includes(q)))
    return offers.length ? [{ ...card, offers }] : []
  })
}

/** 单价已包含服务档策略；仅应用服务端解析的分组倍率和展示单位。 */
export function formatCatalogPrice(value: number | null, multiplier: number, unit: string): string {
  if (value === null || !Number.isFinite(value) || value < 0 || !Number.isFinite(multiplier) || multiplier < 0) return '—'
  if (unit !== 'USD/token' && unit !== 'USD/request') return '—'
  return formatScaled(value, multiplier * (unit === 'USD/token' ? 1_000_000 : 1))
}
