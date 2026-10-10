import { describe, expect, it, vi } from 'vitest'
import { getAvailable, getCatalog } from '../channels'
import { apiClient } from '../client'
vi.mock('../client', () => ({ apiClient: { get: vi.fn() } }))

describe('可用渠道客户端兼容', () => {
  it('旧数组调用不选择目录，新目录只传单个精确 view 且传递取消信号', async () => {
    const signal = new AbortController().signal
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: [] }).mockResolvedValueOnce({ data: { groups: [], user_rate_status: 'not_requested' } })
    expect(await getAvailable()).toEqual([])
    expect(apiClient.get).toHaveBeenNthCalledWith(1, '/channels/available', { signal: undefined })
    expect(await getCatalog({ signal })).toEqual({ groups: [], user_rate_status: 'not_requested' })
    expect(apiClient.get).toHaveBeenNthCalledWith(2, '/channels/available', { params: { view: 'catalog' }, signal })
  })
})
