/**
 * User Channels API endpoints (non-admin)
 * 用户侧「可用渠道」聚合查询：渠道 + 用户可访问的分组 + 支持模型（含定价）。
 */

import { apiClient } from './client'
import type { BillingMode } from '@/constants/channel'

export interface UserAvailableGroup {
  id: number
  name: string
  platform: string
  /** 'standard' | 'subscription' — 订阅分组视觉加深，和 API 密钥页保持一致。 */
  subscription_type: string
  /** 分组默认倍率。用户专属倍率（若有）通过 /groups/rates 获取后在前端 join。 */
  rate_multiplier: number
  peak_rate_enabled: boolean
  peak_start: string
  peak_end: string
  peak_rate_multiplier: number
  /** true = 专属分组（小范围授权）；false = 公开分组。 */
  is_exclusive: boolean
}

export interface UserPricingInterval {
  min_tokens: number
  max_tokens: number | null
  tier_label?: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
  input_multiplier?: number | null
  output_multiplier?: number | null
  cache_write_multiplier?: number | null
  cache_read_multiplier?: number | null
  per_request_price: number | null
}

export interface UserSupportedModelPricing {
  billing_mode: BillingMode
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
  reasoning_effort_multipliers?: Record<string, number> | null
  image_input_price: number | null
  image_output_price: number | null
  per_request_price: number | null
  intervals: UserPricingInterval[]
}

export interface UserSupportedModel {
  name: string
  platform: string
  pricing: UserSupportedModelPricing | null
}

/**
 * 渠道下单个平台的子视图：用户可访问的分组 + 该平台支持的模型。
 * 后端把一个渠道按平台聚合成 sections，前端可以把渠道名作为 row-group
 * 一次渲染，后面按 sections 顺序用 rowspan 铺开。
 */
export interface UserChannelPlatformSection {
  platform: string
  groups: UserAvailableGroup[]
  supported_models: UserSupportedModel[]
}

export interface UserAvailableChannel {
  name: string
  description: string
  platforms: UserChannelPlatformSection[]
}

/** 列出当前用户可见的「可用渠道」（与 /groups/available 保持一致，返回平数组）。 */
export async function getAvailable(options?: { signal?: AbortSignal }): Promise<UserAvailableChannel[]> {
  const { data } = await apiClient.get<UserAvailableChannel[]>('/channels/available', {
    signal: options?.signal
  })
  return data
}

/** 显式选择目录；旧 getAvailable 继续返回渠道数组。 */
export async function getCatalog(options?: { signal?: AbortSignal }): Promise<AvailableModelCatalog> {
  const { data } = await apiClient.get<AvailableModelCatalog>('/channels/available', {
    params: { view: 'catalog' },
    signal: options?.signal
  })
  return data
}

export const userChannelsAPI = { getAvailable, getCatalog }

export default userChannelsAPI


export interface AvailableModelCatalog {
  groups: CatalogGroup[]
  user_rate_status: 'loaded' | 'unavailable' | 'not_requested'
}

export interface CatalogGroup extends UserAvailableGroup {
  description: string
  user_rate_multiplier: number | null
  long_context_pricing_enabled: boolean
  image_rate_independent: boolean
  image_rate_multiplier: number
  video_rate_independent: boolean
  video_rate_multiplier: number
  rate_multipliers: {
    token: number
    image: number
    video: number
    reference_only: boolean
    pricing_at: string
    timezone: string
  }
  models: CatalogOffer[]
}

export interface CatalogOffer {
  offer_key: string
  name: string
  platform: string
  source: { name: string; description: string }
  billing_mode: string | null
  billing_unit: string
  price_status: 'resolved' | 'unknown'
  price_reason: 'pricing_unavailable' | 'request_dependent' | 'unsupported_unit' | null
  pricing: CatalogPricing | null
}

export interface CatalogTokenPrice {
  min_tokens: number
  max_tokens: number | null
  tier_label: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
}

export interface CatalogRequestTier {
  min_tokens: number
  max_tokens: number | null
  label: string
  price: number | null
  falls_back_to_context: boolean
}

export interface CatalogPricing {
  reference_at: string
  reference_only: boolean
  service_tiers: {
    service_tier: string
    context: { basis: string; intervals: CatalogTokenPrice[] } | null
  }[]
  request_pricing: {
    default_price: number | null
    context_tiers: CatalogRequestTier[]
    size_tiers: CatalogRequestTier[]
  } | null
  time_pricing: {
    timezone: string
    weekdays_only: boolean
    periods: { start_time: string; end_time: string; multiplier: number }[]
  } | null
  reasoning_effort_multipliers: Record<string, number>
  unsupported_components: string[]
}
