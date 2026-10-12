import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'

import VersionBadge from '../VersionBadge.vue'

const appStore = reactive({
  versionLoading: false,
  versionLoaded: true,
  versionWarning: '',
  currentVersion: '0.2.7-klno.4',
  latestVersion: '0.2.7-klno.4',
  hasUpdate: false,
  releaseInfo: null as null | Record<string, string>,
  buildType: 'release',
  updateMode: 'in_place',
  upstreamVersion: null as null | Record<string, unknown>,
  fetchVersion: vi.fn(),
  clearVersionCache: vi.fn()
})
const authStore = reactive({ isAdmin: true })

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) =>
        params?.version ? `${key}:${params.version}` : key
    })
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() })
}))

const { performUpdate, getRollbackVersions, rollback } = vi.hoisted(() => ({
  performUpdate: vi.fn(),
  getRollbackVersions: vi.fn(),
  rollback: vi.fn()
}))

vi.mock('@/api/admin/system', () => ({
  performUpdate,
  getRollbackVersions,
  rollback,
  restartService: vi.fn()
}))

describe('VersionBadge', () => {
  beforeEach(() => {
    authStore.isAdmin = true
    appStore.currentVersion = '0.2.7-klno.4'
    appStore.updateMode = 'in_place'
    appStore.buildType = 'release'
    appStore.latestVersion = '0.2.7-klno.4'
    appStore.versionLoaded = true
    appStore.versionWarning = ''
    appStore.releaseInfo = null
    appStore.hasUpdate = false
    appStore.upstreamVersion = null
    vi.clearAllMocks()
    appStore.clearVersionCache.mockImplementation(() => {
      appStore.versionLoaded = false
      appStore.hasUpdate = false
    })
  })

  it('shows upstream X.Y.Z and the fork klno sequence; an upstream release lights the original amber ping', () => {
    appStore.upstreamVersion = {
      current_version: '0.2.7',
      latest_version: '0.2.8',
      has_update: true,
      html_url: 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.8'
    }
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })

    const upstream = wrapper.get('[data-testid="version-upstream"]')
    expect(upstream.element.tagName).toBe('A')
    expect(upstream.attributes('href')).toBe(
      'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.8'
    )
    expect(upstream.text()).toBe('v0.2.7')
    expect(upstream.attributes('title')).toBe(
      'version.upstreamLabel v0.2.7：version.upstreamUpdateAvailable:0.2.8'
    )
    expect(upstream.find('.animate-ping').exists()).toBe(true)

    const fork = wrapper.get('[data-testid="version-fork"]')
    expect(fork.text()).toBe('klno.4')
    expect(fork.attributes('title')).toBe('version.forkLabel v0.2.7-klno.4：version.upToDate')
  })

  it('derives the upstream version before the update check returns', () => {
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })

    const upstream = wrapper.get('[data-testid="version-upstream"]')
    expect(upstream.text()).toBe('v0.2.7')
    expect(upstream.attributes('href')).toBe('https://github.com/Wei-Shaw/sub2api/releases')
    expect(upstream.find('.animate-ping').exists()).toBe(false)
  })

  it('shows the full version on builds without a klno suffix', () => {
    appStore.currentVersion = '0.2.8'
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })

    expect(wrapper.get('[data-testid="version-fork"]').text()).toBe('v0.2.8')
    expect(wrapper.get('[data-testid="version-upstream"]').text()).toBe('v0.2.8')
  })

  it('manual rollback commands point at the fork, not upstream', async () => {
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })
    const vm = wrapper.vm as unknown as {
      selectedRollbackVersion: string
      scriptRollbackCommand: string
      dockerRollbackCommand: string
    }
    vm.selectedRollbackVersion = '0.2.7-klno.3'
    await wrapper.vm.$nextTick()

    expect(vm.scriptRollbackCommand).toContain('raw.githubusercontent.com/KlN-4096/sub2api/v0.2.7-klno.3/')
    expect(vm.dockerRollbackCommand).toContain('image: ghcr.io/kln-4096/sub2api:0.2.7-klno.3')
  })

  it('shows the personal suffix and base version without offering a binary update', async () => {
    appStore.currentVersion = '0.2.14-klno.5-tps.1'
    appStore.latestVersion = '0.2.14-klno.5-tps.2'
    appStore.hasUpdate = true
    appStore.updateMode = 'container'
    appStore.releaseInfo = { html_url: 'https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.5-tps.2' }
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })

    expect(wrapper.get('[data-testid="version-upstream"]').text()).toBe('v0.2.14')
    expect(wrapper.get('[data-testid="version-fork"]').text()).toBe('klno.5-tps.1')
    await wrapper.get('[data-testid="version-fork"]').trigger('click')
    expect(wrapper.text()).toContain('version.containerModeHint')
    expect(wrapper.findAll('button').some(button => button.text() === 'version.updateNow')).toBe(false)
    expect(wrapper.findAll('button').some(button => button.text() === 'version.rollback')).toBe(false)
    expect(wrapper.get('a[href*="ccisnoxx"]').attributes('href')).toContain('tps.2')
    expect(wrapper.get('[data-testid="version-fork"]').attributes('title')).toContain('version.personalLabel')
    expect(performUpdate).not.toHaveBeenCalled()
    expect(getRollbackVersions).not.toHaveBeenCalled()
    expect(rollback).not.toHaveBeenCalled()
  })

  it('does not claim the latest version when the check failed or has not completed', async () => {
    appStore.versionLoaded = false
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })
    await wrapper.get('[data-testid="version-fork"]').trigger('click')
    expect(wrapper.text()).toContain('version.checkPending')
    expect(wrapper.text()).not.toContain('version.upToDate')

    appStore.versionLoaded = true
    appStore.versionWarning = 'GitHub unavailable'
    appStore.hasUpdate = true
    appStore.upstreamVersion = {
      current_version: '0.2.7', latest_version: '0.2.8', has_update: true
    }
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('version.checkFailed')
    expect(wrapper.text()).not.toContain('version.upToDate')
    expect(wrapper.get('[data-testid="version-fork"]').attributes('title')).toContain('version.checkFailed')
    expect(wrapper.find('.animate-ping').exists()).toBe(false)
  })

  it('keeps the source update hint for tagged personal source builds', async () => {
    appStore.currentVersion = '0.2.14-klno.5-tps.1'
    appStore.latestVersion = '0.2.14-klno.5-tps.2'
    appStore.hasUpdate = true
    appStore.buildType = 'source'
    appStore.updateMode = 'manual'
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })

    await wrapper.get('[data-testid="version-fork"]').trigger('click')
    expect(wrapper.text()).toContain('version.sourceModeHint')
    expect(wrapper.text()).not.toContain('version.containerModeHint')
    expect(wrapper.findAll('button').some(button => button.text() === 'version.updateNow')).toBe(false)
    expect(performUpdate).not.toHaveBeenCalled()
  })

  it('reports an upstream check failure without an update indicator', () => {
    appStore.upstreamVersion = {
      current_version: '0.2.7', latest_version: '0.2.8', has_update: true, warning: 'GitHub unavailable'
    }
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })
    expect(wrapper.get('[data-testid="version-upstream"]').attributes('title')).toContain('version.checkFailed')
    expect(wrapper.get('[data-testid="version-upstream"]').find('.animate-ping').exists()).toBe(false)
  })

  it('keeps the update button for ordinary KlN release builds', async () => {
    appStore.hasUpdate = true
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })
    await wrapper.get('[data-testid="version-fork"]').trigger('click')
    expect(wrapper.findAll('button').some(button => button.text() === 'version.updateNow')).toBe(true)
    expect(wrapper.text()).not.toContain('version.containerModeHint')
  })

  it('keeps the restart action after a successful ordinary binary update clears the cache', async () => {
    appStore.hasUpdate = true
    performUpdate.mockResolvedValueOnce({ need_restart: true })
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: true } } })
    await wrapper.get('[data-testid="version-fork"]').trigger('click')
    await wrapper.findAll('button').find(button => button.text() === 'version.updateNow')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('version.updateComplete')
    expect(wrapper.text()).toContain('version.restartNow')
    expect(appStore.clearVersionCache).toHaveBeenCalled()
  })
})
