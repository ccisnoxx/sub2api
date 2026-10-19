import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AccountTableFilters from '../AccountTableFilters.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import type { AdminGroup } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: { count: number }) => params ? `${key} (${params.count})` : key
    })
  }
})

const defaultFilters = () => ({
  platform: '', type: '', status: '', privacy_mode: '', group: '',
  search: 'existing search', lite: '1', sort_by: 'priority', sort_order: 'asc'
})

function mountFilters(filters: Record<string, unknown> = defaultFilters()) {
  return mount(AccountTableFilters, {
    attachTo: document.body,
    props: {
      searchQuery: 'existing search', filters,
      groups: [{ id: 42, name: 'A very long group name' } as AdminGroup]
    },
    global: { stubs: { Teleport: true } }
  })
}

enableAutoUnmount(afterEach)
afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('AccountTableFilters', () => {
  it('keeps all filters visible without emitting changes or clearing selected values', () => {
    const filters = { ...defaultFilters(), type: 'bedrock', privacy_mode: '__unset__', group: 'ungrouped' }
    const wrapper = mountFilters(filters)
    expect(wrapper.findAllComponents(Select)).toHaveLength(5)
    expect(wrapper.findAllComponents(Select).every(select => select.isVisible())).toBe(true)
    expect(wrapper.props('filters')).toEqual(filters)
    expect(wrapper.emitted('update:filters')).toBeUndefined()
    expect(wrapper.emitted('change')).toBeUndefined()
  })

  it('reacts to external filter updates and resets without emitting its own changes', async () => {
    const wrapper = mountFilters()
    const keys = ['platform', 'type', 'status', 'privacy_mode', 'group'] as const
    for (const filters of [
      { ...defaultFilters(), platform: 'openai', type: 'oauth', status: 'active', privacy_mode: '__unset__', group: '42' },
      defaultFilters()
    ]) {
      await wrapper.setProps({ filters })
      for (const [index, key] of keys.entries()) {
        expect(wrapper.findAllComponents(Select)[index].props('modelValue')).toBe(filters[key])
      }
    }
    expect(wrapper.emitted('update:filters')).toBeUndefined()
    expect(wrapper.emitted('change')).toBeUndefined()
  })

  it('preserves every option and emits a merged filter snapshot plus change for every selection', async () => {
    const filters = { ...defaultFilters(), platform: 'openai', type: 'oauth', status: 'active', privacy_mode: '__unset__', group: '42' }
    const wrapper = mountFilters(filters)
    const expectedOptions = [
      ['platform', ['', ...CONCRETE_PLATFORM_OPTIONS.map(option => option.value)]],
      ['type', ['', 'oauth', 'setup-token', 'apikey', 'cpr', 'bedrock']],
      ['status', ['', 'active', 'inactive', 'error', 'rate_limited', 'temp_unschedulable', 'unschedulable']],
      ['privacy_mode', ['', '__unset__', 'training_off', 'training_set_cf_blocked', 'training_set_failed']],
      ['group', ['', 'ungrouped', '42']]
    ] as const
    let changes = 0
    for (const [index, [key, values]] of expectedOptions.entries()) {
      const select = wrapper.findAllComponents(Select)[index]
      expect(select.props('options').map(option => option.value)).toEqual(values)
      expect(select.props('modelValue')).toBe(filters[key])
      expect(select.get('button').attributes('aria-label')).not.toBe('Select option')
      for (const [optionIndex, value] of values.entries()) {
        await select.get('button').trigger('click')
        await select.findAll('[role="option"]')[optionIndex].trigger('click')
        changes += 1
        expect(wrapper.emitted('update:filters')?.at(-1)).toEqual([{ ...filters, [key]: value }])
        expect(wrapper.emitted('change')).toHaveLength(changes)
        expect(wrapper.props('filters')).toEqual(filters)
      }
    }
    expect(wrapper.findAllComponents(Select)[4].props('options')[2].label).toBe('A very long group name')
    wrapper.unmount()
  })

  it('preserves immediate search updates and the debounced search change event', async () => {
    vi.useFakeTimers()
    const wrapper = mountFilters()
    const search = wrapper.getComponent(SearchInput)
    expect(search.props('modelValue')).toBe('existing search')
    expect(search.props('placeholder')).toBe('admin.accounts.searchAccounts')
    await search.get('input').setValue('new search')
    expect(wrapper.emitted('update:searchQuery')).toEqual([['new search']])
    expect(wrapper.emitted('change')).toBeUndefined()
    await vi.advanceTimersByTimeAsync(300)
    expect(wrapper.emitted('change')).toEqual([[]])
    expect(wrapper.emitted('update:filters')).toBeUndefined()
    wrapper.unmount()
  })

  it('gives every filter a distinct accessible name', () => {
    const wrapper = mountFilters()
    const names = wrapper.findAllComponents(Select).map(select => select.get('button').attributes('aria-label'))
    expect(names).toEqual([
      'admin.accounts.allPlatforms', 'admin.accounts.allTypes', 'admin.accounts.allStatus',
      'admin.accounts.allPrivacyModes', 'admin.accounts.allGroups'
    ])
    expect(new Set(names).size).toBe(5)
  })
})
