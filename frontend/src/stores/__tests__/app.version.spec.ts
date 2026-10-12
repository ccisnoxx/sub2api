import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { checkUpdates, type VersionInfo } from '@/api/admin/system'
import { useAppStore } from '@/stores/app'

vi.mock('@/api/admin/system', () => ({ checkUpdates: vi.fn() }))
vi.mock('@/api/auth', () => ({ getPublicSettings: vi.fn() }))

function personalVersion(overrides: Partial<VersionInfo> = {}): VersionInfo {
  return {
    current_version: '0.2.15-klno.2-tps.1',
    latest_version: '0.2.15-klno.2-tps.2',
    has_update: true,
    build_type: 'release',
    update_mode: 'container',
    release_repository: 'ccisnoxx/sub2api',
    cached: false,
    upstream: { current_version: '0.2.15', latest_version: '0.2.16', has_update: true },
    ...overrides
  }
}

describe('个人版本缓存', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(checkUpdates).mockReset()
  })

  it('缓存保留来源、更新模式与 warning，清除后使用新的安装能力', async () => {
    const store = useAppStore()
    const info = personalVersion({ warning: 'Using cached data: GitHub API returned 503' })
    vi.mocked(checkUpdates).mockResolvedValueOnce(info)

    expect(await store.fetchVersion()).toEqual(info)
    expect(await store.fetchVersion()).toEqual({ ...info, cached: true })
    expect(checkUpdates).toHaveBeenCalledTimes(1)
    expect(store.updateMode).toBe('container')
    expect(store.releaseRepository).toBe('ccisnoxx/sub2api')
    expect(store.versionWarning).toBe(info.warning)

    store.clearVersionCache()
    expect(store.versionLoaded).toBe(false)
    expect(store.hasUpdate).toBe(false)
    expect(store.versionWarning).toBe('')
    vi.mocked(checkUpdates).mockResolvedValueOnce(personalVersion({
      build_type: 'source', update_mode: 'manual'
    }))
    await store.fetchVersion()
    expect(store.updateMode).toBe('manual')
    expect(store.versionWarning).toBe('')
  })

  it('刷新失败标记旧结果为不可确认，恢复成功后清除失败状态', async () => {
    const store = useAppStore()
    vi.mocked(checkUpdates).mockResolvedValueOnce(personalVersion())
    await store.fetchVersion()
    vi.mocked(checkUpdates).mockRejectedValueOnce(new Error('GitHub unavailable'))
    const log = vi.spyOn(console, 'error').mockImplementation(() => {})
    try {
      expect(await store.fetchVersion(true)).toBeNull()
      expect(store.versionWarning).toBe('VERSION_CHECK_FAILED')
      expect((await store.fetchVersion())?.warning).toBe('VERSION_CHECK_FAILED')

      vi.mocked(checkUpdates).mockResolvedValueOnce(personalVersion({ has_update: false }))
      await store.fetchVersion(true)
      expect(store.versionWarning).toBe('')
      expect(store.hasUpdate).toBe(false)
    } finally {
      log.mockRestore()
    }
  })
})
