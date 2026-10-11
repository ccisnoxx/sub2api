<template>
  <details ref="details" class="min-w-0 rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800" @keydown.esc.stop.prevent="closeDetails">
    <summary ref="summary" class="cursor-pointer rounded-xl p-4 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500">
      <span class="ml-1 break-all font-semibold text-gray-900 dark:text-white">{{ model.name }}</span>
      <span class="ml-2 text-xs uppercase text-gray-500 dark:text-gray-400">{{ model.platform }}</span>
      <span class="mt-2 block text-sm text-gray-500 dark:text-gray-400">{{ t('availableChannels.catalog.offerCount', { count: model.offers.length }) }}</span>
    </summary>
    <div v-if="entry" class="min-w-0 space-y-4 border-t border-gray-100 p-4 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-300">
      <label class="block">
        <span class="mb-1 block font-medium">{{ t('availableChannels.catalog.groupOffer') }}</span>
        <select v-model="selectedOffer" class="input w-full" data-testid="catalog-offer">
          <option v-for="item in model.offers" :key="item.offer.offer_key" :value="item.offer.offer_key">{{ item.group.name }} · {{ item.offer.source.name }}</option>
        </select>
      </label>
      <p class="break-words">{{ entry.group.description }}</p>
      <p class="break-words">{{ t('availableChannels.catalog.source') }}: {{ entry.offer.source.name }} <span v-if="entry.offer.source.description">— {{ entry.offer.source.description }}</span></p>
      <div class="flex flex-wrap gap-2 text-xs">
        <span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-700">{{ t(entry.group.is_exclusive ? 'availableChannels.exclusive' : 'availableChannels.public') }}</span>
        <span v-if="entry.group.subscription_type === 'subscription'" class="rounded bg-purple-50 px-2 py-1 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300">{{ t('availableChannels.catalog.subscription') }}</span>
      </div>
      <dl class="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <div><dt class="text-gray-500">{{ t('availableChannels.pricing.billingMode') }}</dt><dd>{{ modeLabel }}</dd></div>
        <div><dt class="text-gray-500">{{ t('availableChannels.catalog.billingUnit') }}</dt><dd>{{ unitLabel }}</dd></div>
        <div><dt class="text-gray-500">{{ t('availableChannels.catalog.defaultRate') }}</dt><dd>×{{ entry.group.rate_multiplier }}</dd></div>
        <div><dt class="text-gray-500">{{ t('availableChannels.catalog.personalRate') }}</dt><dd>{{ userRateStatus === 'unavailable' ? t('availableChannels.catalog.rateUnavailableShort') : entry.group.user_rate_multiplier === null ? t('availableChannels.catalog.noOverride') : `×${entry.group.user_rate_multiplier}` }}</dd></div>
        <div><dt class="text-gray-500">{{ t('availableChannels.catalog.appliedRate') }}</dt><dd data-testid="catalog-rate">×{{ entry.group.rate_multipliers.token }}</dd></div>
      </dl>
      <p v-if="entry.group.peak_rate_enabled" class="text-xs text-amber-700 dark:text-amber-300">{{ t('availableChannels.catalog.peakRule', { start: entry.group.peak_start, end: entry.group.peak_end, multiplier: entry.group.peak_rate_multiplier, timezone: entry.group.rate_multipliers.timezone }) }}</p>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('availableChannels.catalog.snapshot', { at: entry.group.rate_multipliers.pricing_at, timezone: entry.group.rate_multipliers.timezone }) }}</p>
      <p v-if="entry.offer.price_status === 'unknown' || !pricing" role="status" class="rounded-lg bg-amber-50 p-3 text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ unknownLabel }}</p>
      <template v-else>
        <p class="rounded-lg bg-primary-50 p-3 text-xs leading-5 text-primary-800 dark:bg-primary-900/20 dark:text-primary-200">{{ t('availableChannels.catalog.referenceNotice') }}</p>
        <label v-if="pricing.service_tiers.length" class="block">
          <span class="mb-1 block font-medium">{{ t('availableChannels.catalog.serviceTier') }}</span>
          <select v-model="selectedTier" class="input" data-testid="catalog-tier">
            <option v-for="tier in pricing.service_tiers" :key="tier.service_tier" :value="tier.service_tier">{{ tier.service_tier }}</option>
          </select>
        </label>
        <template v-if="context">
          <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('availableChannels.catalog.contextRule') }}</p>
          <div class="overflow-x-auto" tabindex="0" :aria-label="t('availableChannels.catalog.priceTable')">
            <table class="w-full text-left text-xs">
              <caption class="mb-2 text-left font-medium">{{ t('availableChannels.catalog.priceTable') }} · {{ unitLabel }}</caption>
              <thead><tr><th class="whitespace-nowrap p-2">{{ t('availableChannels.catalog.context') }}</th><th v-for="column in tokenColumns" :key="column.key" class="whitespace-nowrap p-2">{{ t(`availableChannels.pricing.${column.label}`) }}</th></tr></thead>
              <tbody><tr v-for="(interval, index) in context.intervals" :key="index" class="border-t border-gray-100 dark:border-dark-700">
                <td class="whitespace-nowrap p-2">{{ range(interval.min_tokens, interval.max_tokens) }} <span v-if="interval.tier_label">{{ interval.tier_label }}</span></td>
                <td v-for="column in tokenColumns" :key="column.key" class="whitespace-nowrap p-2" :data-price="column.key">{{ price(interval[column.key]) }}</td>
              </tr></tbody>
            </table>
          </div>
          <p class="text-xs text-gray-500">{{ t('availableChannels.catalog.missingPrice') }}</p>
        </template>
        <div v-if="pricing.request_pricing" class="space-y-2">
          <p>{{ t('availableChannels.catalog.defaultPrice') }}: {{ price(pricing.request_pricing.default_price) }} · {{ unitLabel }}</p>
          <p v-for="(tier, index) in pricing.request_pricing.context_tiers" :key="`context-${index}`">{{ range(tier.min_tokens, tier.max_tokens) }}: {{ price(tier.price) }}</p>
          <p v-for="(tier, index) in pricing.request_pricing.size_tiers" :key="`size-${index}`">{{ tier.label }}: {{ tier.falls_back_to_context ? t('availableChannels.catalog.contextFallback') : price(tier.price) }}</p>
        </div>
        <div v-if="pricing.time_pricing" class="space-y-1 text-xs">
          <p class="font-medium">{{ t('availableChannels.catalog.timeRule') }} · {{ pricing.time_pricing.timezone }} · {{ t(pricing.time_pricing.weekdays_only ? 'availableChannels.catalog.weekdays' : 'availableChannels.catalog.everyday') }}</p>
          <p v-for="(period, index) in pricing.time_pricing.periods" :key="index">{{ period.start_time }}–{{ period.end_time }} ×{{ period.multiplier }}</p>
        </div>
        <div v-if="Object.keys(pricing.reasoning_effort_multipliers).length" class="space-y-1 text-xs">
          <p class="font-medium">{{ t('availableChannels.catalog.effortRule') }}</p>
          <div class="flex flex-wrap gap-2"><span v-for="(multiplier, effort) in pricing.reasoning_effort_multipliers" :key="effort">{{ effort }} ×{{ multiplier }}</span></div>
        </div>
        <p v-if="pricing.unsupported_components.length" class="text-xs text-amber-700 dark:text-amber-300">{{ t('availableChannels.catalog.unsupportedComponents') }}: {{ pricing.unsupported_components.map(componentLabel).join(' · ') }}</p>
        <p class="break-all text-xs text-gray-500">{{ t('availableChannels.catalog.priceAt', { at: pricing.reference_at }) }}</p>
      </template>
    </div>
  </details>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AvailableModelCatalog, CatalogTokenPrice } from '@/api/channels'
import { formatCatalogPrice, type ModelCatalogCard } from '@/utils/modelCatalog'

const props = defineProps<{ model: ModelCatalogCard; userRateStatus: AvailableModelCatalog['user_rate_status'] }>()
const { t } = useI18n()
const offerKey = ref('')
const tierName = ref('default')
const selectedOffer = computed({
  get: () => props.model.offers.some(item => item.offer.offer_key === offerKey.value) ? offerKey.value : props.model.offers[0]?.offer.offer_key ?? '',
  set: (value: string) => { offerKey.value = value }
})
const selectedTier = computed({
  get: () => pricing.value?.service_tiers.some(tier => tier.service_tier === tierName.value) ? tierName.value : pricing.value?.service_tiers[0]?.service_tier ?? '',
  set: (value: string) => { tierName.value = value }
})
const details = ref<HTMLDetailsElement>()
const summary = ref<HTMLElement>()
const entry = computed(() => props.model.offers.find(item => item.offer.offer_key === selectedOffer.value) ?? props.model.offers[0])
const pricing = computed(() => entry.value?.offer.pricing)
const context = computed(() => (pricing.value?.service_tiers.find(tier => tier.service_tier === selectedTier.value) ?? pricing.value?.service_tiers[0])?.context)
const unitLabel = computed(() => {
  const unit = entry.value?.offer.billing_unit
  if (unit === 'USD/token') return `USD ${t('availableChannels.pricing.unitPerMillion')}`
  if (unit === 'USD/request') return `USD ${t('availableChannels.pricing.unitPerRequest')}`
  return t('availableChannels.catalog.unknownUnit')
})
const modeLabel = computed(() => {
  const keys: Record<string, string> = { token: 'billingModeToken', per_request: 'billingModePerRequest', image: 'billingModeImage', video: 'billingModeVideo' }
  const key = keys[entry.value?.offer.billing_mode ?? '']
  return key ? t(`availableChannels.pricing.${key}`) : t('availableChannels.catalog.unknownUnit')
})
const unknownLabel = computed(() => {
  const keys = { pricing_unavailable: 'unknownPrice', request_dependent: 'requestDependent', unsupported_unit: 'unsupportedUnit' }
  const reason = entry.value?.offer.price_reason
  return t(`availableChannels.catalog.${reason ? keys[reason] : 'unknownPrice'}`)
})
type TokenPriceKey = keyof Pick<CatalogTokenPrice, 'input_price' | 'output_price' | 'cache_write_price' | 'cache_write_1h_price' | 'cache_read_price'>
const tokenColumns: { key: TokenPriceKey; label: string }[] = [
  { key: 'input_price', label: 'inputPrice' }, { key: 'output_price', label: 'outputPrice' },
  { key: 'cache_write_price', label: 'cacheWrite5mPrice' }, { key: 'cache_write_1h_price', label: 'cacheWrite1hPrice' },
  { key: 'cache_read_price', label: 'cacheReadPrice' }
]
function price(value: number | null) {
  return entry.value ? formatCatalogPrice(value, entry.value.group.rate_multipliers.token, entry.value.offer.billing_unit) : '—'
}
function range(min: number, max: number | null) { return `(${min}, ${max === null ? '∞' : max}]` }
function componentLabel(component: string) {
  const keys: Record<string, string> = { image_input_token: 'imageInputPrice', image_output_token: 'imageOutputPrice', image_cache_read_token: 'cacheReadPrice' }
  return component === 'audio' ? t('availableChannels.catalog.audio') : keys[component] ? t(`availableChannels.pricing.${keys[component]}`) : component
}
function closeDetails() {
  if (details.value) details.value.open = false
  summary.value?.focus()
}
</script>
