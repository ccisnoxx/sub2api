import { describe, expect, it } from 'vitest'
import {
  averageUsageTps, displayedFirstTokenMs, strictFirstTokenMs,
  usageCompletionStatus, usageTimingExportValues, type UsageTimingRow,
} from '../usageTiming'

const textRow: UsageTimingRow = {
  output_tokens: 500, duration_ms: 10000, first_token_ms: 9000,
  image_count: 0, image_output_tokens: 0, billing_mode: 'token',
  native_compaction_v2: false, request_type: 'stream', stream: true,
}
const newRow: UsageTimingRow = { ...textRow, timing_version: 1, audio_output_tokens: 0 }
const t = (key: string) => key

describe('用量计时共享合同', () => {
  it.each([
    { strict_first_token_ms: null, last_token_ms: null },
    { strict_first_token_ms: 9990, last_token_ms: 9991 },
    { strict_first_token_ms: 500, last_token_ms: 500 },
    { strict_first_token_ms: 9999, last_token_ms: 2 },
  ])('普通文本始终用完整总耗时，首末时点 %j 不改变 TPS', (times) => {
    expect(averageUsageTps({ ...newRow, ...times })).toEqual({ value: 50, reason: null, partial: false })
  })

  it('历史普通文本可计算，但旧首字不能成为严格首 Token', () => {
    expect(averageUsageTps(textRow).value).toBe(50)
    expect(displayedFirstTokenMs(textRow)).toBe(9000)
    expect(strictFirstTokenMs({ ...textRow, strict_first_token_ms: 10 })).toBeNull()
    expect(displayedFirstTokenMs({ ...newRow, strict_first_token_ms: null })).toBeNull()
    expect(displayedFirstTokenMs({ ...newRow, strict_first_token_ms: 0 })).toBe(0)
  })

  it('新混合输出减去可信媒体 Token', () => {
    const row = { ...newRow, output_tokens: 1000, image_count: 1, image_output_tokens: 300, audio_output_tokens: 200 }
    expect(averageUsageTps(row)).toEqual({ value: 50, reason: null, partial: false })
  })

  it.each([null, undefined])('新记录音频未知 %s 不得解释为 0', (audio_output_tokens) => {
    expect(averageUsageTps({ ...newRow, audio_output_tokens }).reason).toBe('usage.tpsMediaUnknown')
  })

  it('图片存在而 Token 拆分缺失时保留未知', () => {
    expect(averageUsageTps({ ...newRow, image_count: 1 }).reason).toBe('usage.tpsMediaUnknown')
  })

  it.each([
    { image_output_tokens: -1 }, { image_output_tokens: NaN }, { image_output_tokens: Infinity },
    { audio_output_tokens: -1 }, { audio_output_tokens: NaN }, { audio_output_tokens: Infinity },
    { image_output_tokens: 300, audio_output_tokens: 201 },
  ])('无效媒体统计 %j 不被吞掉或钳成 0', (counts) => {
    const result = averageUsageTps({ ...newRow, ...counts })
    expect(result.value).toBeNull()
    expect(result.reason).toBe('usage.tpsInvalidMedia')
  })

  it.each([
    { image_output_tokens: 500 }, { audio_output_tokens: 500 },
    { first_output_kind: 'compaction' as const }, { native_compaction_v2: true },
    { request_type: 'live' as const }, { request_type: 'probe' as const }, { request_type: 'gwpool_degraded' as const },
  ])('纯媒体或不适用记录 %j 排除文本 TPS', (fields) => {
    expect(averageUsageTps({ ...newRow, ...fields }).reason).toBe('usage.tpsNotApplicable')
  })

  it('压缩之后实际观察到文本 Token 才可计算文本平均速率', () => {
    expect(averageUsageTps({ ...newRow, first_output_kind: 'compaction', strict_first_token_ms: 50 }).value).toBe(50)
  })

  it.each(['client_disconnected', 'upstream_error', 'interrupted'] as const)('有效 %s 记录仍可计算并标注部分响应', (completion_status) => {
    const row = { ...newRow, completion_status, is_complete: false, usage_source: 'upstream_partial' as const }
    expect(averageUsageTps(row)).toEqual({ value: 50, reason: null, partial: true })
    expect(usageCompletionStatus(row)).toBe(completion_status)
  })

  it('未知状态和未知来源不会从 Token 或扣费推断成功或部分', () => {
    expect(usageCompletionStatus(textRow)).toBe('unknown')
    expect(averageUsageTps({ ...newRow, is_complete: null, usage_source: 'unknown' }).partial).toBe(false)
    expect(averageUsageTps({ ...newRow, usage_source: 'upstream_partial' }).partial).toBe(true)
  })

  it('导出保留未经取整的平均速率、旧/严格时点区别和 false/空值', () => {
    const row = {
      ...newRow, output_tokens: 101, duration_ms: 345, image_count: 1,
      image_output_tokens: 30, audio_output_tokens: 20, strict_first_token_ms: 300,
      last_token_ms: 302, first_output_ms: 25, first_output_kind: 'image' as const,
      completion_status: 'client_disconnected' as const, is_complete: false, usage_source: 'upstream_partial' as const,
    }
    expect(usageTimingExportValues(row, t)).toEqual([
      147.82608695652175, 'usage.partialResponse', 1, 300, 302, 25, 'image', 30, 20,
      'client_disconnected', false, 'upstream_partial',
    ])
    expect(usageTimingExportValues({ ...newRow, audio_output_tokens: null }, t)).toEqual([
      '', 'usage.tpsMediaUnknown', 1, '', '', '', '', 0, '', 'unknown', '', 'unknown',
    ])
  })
})
