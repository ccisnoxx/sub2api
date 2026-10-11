import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpsErrorDetailModal from '../OpsErrorDetailModal.vue'
import OpsRoutingDiagnosticsPanel from '../OpsRoutingDiagnosticsPanel.vue'
import type { OpsErrorDetail, OpsRoutingDiagnostics } from '@/api/admin/ops'

const mocks = vi.hoisted(() => ({
  getRequestErrorDetail: vi.fn(),
  getUpstreamErrorDetail: vi.fn(),
  listRequestErrorUpstreamErrors: vi.fn()
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getRequestErrorDetail: mocks.getRequestErrorDetail,
    getUpstreamErrorDetail: mocks.getUpstreamErrorDetail,
    listRequestErrorUpstreamErrors: mocks.listRequestErrorUpstreamErrors
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

  beforeEach(() => {
    mocks.getRequestErrorDetail.mockReset()
    mocks.getUpstreamErrorDetail.mockReset()
    mocks.listRequestErrorUpstreamErrors.mockReset()
    mocks.listRequestErrorUpstreamErrors.mockResolvedValue({ items: [] })
  })

describe('OpsErrorDetailModal', () => {

  it('prioritizes upstream root cause and deduplicates diagnostic payloads', async () => {
    mocks.getRequestErrorDetail.mockResolvedValue({
      id: 1,
      created_at: '2026-08-19T00:00:00Z',
      phase: 'request',
      type: 'upstream_error',
      error_owner: 'provider',
      error_source: 'gateway',
      severity: 'P1',
      status_code: 502,
      upstream_status_code: 429,
      platform: 'openai',
      model: 'gpt-5.6',
      resolved: false,
      request_id: 'rid-1',
      message: 'All available accounts exhausted',
      error_body: '{"error":"same"}',
      upstream_error_message: 'provider rate limit exhausted',
      upstream_error_detail: '{"error":"same"}',
      upstream_errors: '[]',
      account_name: 'account',
      group_name: 'group',
      is_business_limited: false
    })

    const wrapper = shallowMount(OpsErrorDetailModal, {
      props: { show: true, errorId: 1, errorType: 'request' },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' },
          Icon: true
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('provider rate limit exhausted')
    expect(wrapper.text()).toContain('admin.ops.errorDetail.upstreamStatus')
    expect(wrapper.text()).toContain('429')
    expect(wrapper.findAll('pre')).toHaveLength(2)
    expect(wrapper.text()).not.toContain('admin.ops.errorDetail.payloads.upstream_detail')
  })
})

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(r => { resolve = r })
  return { promise, resolve }
}

function mountDetail() {
  return shallowMount(OpsErrorDetailModal, {
    props: { show: true, errorId: 1, errorType: 'request' },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Icon: true } }
  })
}

it('ignores a late detail response after selecting another error', async () => {
  const first = deferred<Partial<OpsErrorDetail>>()
  mocks.getRequestErrorDetail.mockReturnValueOnce(first.promise).mockResolvedValueOnce({ id: 2, message: 'current failure' })
  const wrapper = mountDetail()
  await wrapper.setProps({ errorId: 2 })
  await flushPromises()
  expect(wrapper.text()).toContain('current failure')
  first.resolve({ id: 1, message: 'previous failure' })
  await flushPromises()
  expect(wrapper.text()).toContain('current failure')
  expect(wrapper.text()).not.toContain('previous failure')
})

it('passes only the current single-detail snapshot and retains classification', async () => {
  const diagnostics: OpsRoutingDiagnostics = { schema_version: 1, turn: null, selection_attempt: 1,
    candidate_pool: 0, filtered_candidates: 0, filter_reasons: {}, filter_coverage: 'complete',
    selection_layer: 'load_balance', selection_reason: 'pool_empty' }
  mocks.getRequestErrorDetail.mockResolvedValue({ id: 1, phase: 'routing', error_owner: 'platform', error_source: 'gateway', routing_diagnostics: diagnostics })
  const wrapper = mountDetail()
  await flushPromises()
  expect(wrapper.findComponent(OpsRoutingDiagnosticsPanel).props('diagnostics')).toEqual(diagnostics)
  expect(wrapper.text()).toContain('routing')
  expect(wrapper.text()).toContain('platform')
  expect(wrapper.text()).toContain('gateway')
})
it('clears and ignores pending detail and correlated rows after closing', async () => {
  const detail = deferred<Partial<OpsErrorDetail>>()
  const rows = deferred<{ items: Partial<OpsErrorDetail>[] }>()
  mocks.getRequestErrorDetail.mockReturnValue(detail.promise)
  mocks.listRequestErrorUpstreamErrors.mockReturnValue(rows.promise)
  const wrapper = mountDetail()
  await wrapper.setProps({ show: false })
  detail.resolve({ id: 1, message: 'closed detail' })
  rows.resolve({ items: [{ id: 1, message: 'closed row' }] })
  await flushPromises()
  expect(wrapper.text()).not.toContain('closed detail')
  expect(wrapper.text()).not.toContain('closed row')
})
it('reloads the same id when switching request and upstream detail kinds', async () => {
  mocks.getRequestErrorDetail.mockResolvedValue({ id: 1, message: 'request kind' })
  mocks.getUpstreamErrorDetail.mockResolvedValue({ id: 1, message: 'upstream kind' })
  const wrapper = mountDetail()
  await flushPromises()
  await wrapper.setProps({ errorType: 'upstream' })
  await flushPromises()
  expect(mocks.getUpstreamErrorDetail).toHaveBeenCalledWith(1)
  expect(wrapper.text()).toContain('upstream kind')
  expect(wrapper.text()).not.toContain('request kind')
})

it('ignores late correlated rows after switching errors', async () => {
  const first = deferred<{ items: Partial<OpsErrorDetail>[] }>()
  mocks.getRequestErrorDetail.mockResolvedValue({ id: 2, message: 'current error' })
  mocks.listRequestErrorUpstreamErrors.mockReturnValueOnce(first.promise).mockResolvedValueOnce({ items: [{ id: 2, message: 'current correlated' }] })
  const wrapper = mountDetail()
  await wrapper.setProps({ errorId: 2 })
  await flushPromises()
  first.resolve({ items: [{ id: 1, message: 'previous correlated' }] })
  await flushPromises()
  expect(wrapper.text()).toContain('current correlated')
  expect(wrapper.text()).not.toContain('previous correlated')
})
