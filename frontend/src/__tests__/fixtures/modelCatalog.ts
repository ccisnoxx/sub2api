import type { AvailableModelCatalog, CatalogGroup, CatalogOffer } from '@/api/channels'

export function catalogOffer(name = 'GPT-Catalog', key = 'public-offer'): CatalogOffer {
  const interval = { min_tokens: 0, max_tokens: 100000, tier_label: '', input_price: 0.000002, output_price: 0.000008, cache_write_price: null, cache_write_1h_price: 0, cache_read_price: 0.000001 }
  return {
    offer_key: key, name, platform: 'openai', source: { name: 'Catalog channel', description: 'Configured access' },
    billing_mode: 'token', billing_unit: 'USD/token', price_status: 'resolved', price_reason: null,
    pricing: {
      reference_at: '2026-10-09T16:00:00Z', reference_only: true,
      service_tiers: [
        { service_tier: 'default', context: { basis: 'whole_request', intervals: [interval, { ...interval, min_tokens: 100000, max_tokens: null, input_price: 0.000004 }] } },
        { service_tier: 'priority', context: { basis: 'whole_request', intervals: [{ ...interval, input_price: 0.000004 }] } }
      ],
      request_pricing: null,
      time_pricing: { timezone: 'Asia/Shanghai', weekdays_only: true, periods: [{ start_time: '10:00', end_time: '12:00', multiplier: 1.5 }] },
      reasoning_effort_multipliers: { high: 2 }, unsupported_components: ['audio', 'image_output_token']
    }
  }
}

export function catalogGroup(id = 1, name = 'Public group', models = [catalogOffer()]): CatalogGroup {
  return {
    id, name, description: 'Group rules', platform: 'openai', subscription_type: 'standard', is_exclusive: false,
    rate_multiplier: 4, user_rate_multiplier: 0.5, peak_rate_enabled: true, peak_start: '08:00', peak_end: '20:00', peak_rate_multiplier: 1.5,
    long_context_pricing_enabled: true, image_rate_independent: false, image_rate_multiplier: 1,
    video_rate_independent: false, video_rate_multiplier: 1,
    rate_multipliers: { token: 0.75, image: 0.5, video: 0.5, reference_only: false, pricing_at: '2026-10-09T16:00:00Z', timezone: 'Asia/Shanghai' }, models
  }
}

export function catalogFixture(): AvailableModelCatalog {
  const zero = catalogGroup(2, 'Exclusive zero', [catalogOffer('gpt-catalog', 'exclusive-offer')])
  zero.is_exclusive = true
  zero.user_rate_multiplier = 0
  zero.rate_multipliers.token = 0
  const unknown = { ...catalogOffer('Unknown-model', 'unknown-offer'), price_status: 'unknown' as const, price_reason: 'pricing_unavailable' as const, pricing: null }
  const media = { ...unknown, name: 'Audio-model', offer_key: 'media-offer', billing_mode: 'per_request', billing_unit: 'unknown', price_reason: 'unsupported_unit' as const }
  return { user_rate_status: 'loaded', groups: [catalogGroup(1, 'Public group', [catalogOffer(), unknown, media]), zero, catalogGroup(3, 'Empty group', [])] }
}
