import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useAsyncData } from './useAsyncData'

describe('useAsyncData', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads data, merges silent reloads, and handles errors', async () => {
    vi.useFakeTimers()
    let callCount = 0
    const loader = vi.fn(async () => {
      callCount += 1
      if (callCount === 1) {
        return ['alpha']
      }
      if (callCount === 2) {
        return ['beta']
      }
      throw new Error('boom')
    })
    const pollInterval = ref(10)

    const Component = defineComponent({
      setup() {
        return useAsyncData(loader, {
          pollIntervalMs: pollInterval,
          mergeData: (current: string[] | null, incoming: string[]) => [...(current ?? []), ...incoming],
        })
      },
      template: `
        <div>
          <span class="data">{{ JSON.stringify(data) }}</span>
          <span class="loading">{{ String(isLoading) }}</span>
          <span class="refreshing">{{ String(isRefreshing) }}</span>
          <span class="error">{{ error ?? '' }}</span>
        </div>
      `,
    })

    const wrapper = mount(Component)
    await flushPromises()

    expect(loader).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.data').text()).toContain('alpha')
    expect(wrapper.find('.loading').text()).toBe('false')

    await wrapper.vm.reload({ silent: true })
    await flushPromises()

    expect(loader).toHaveBeenCalledTimes(2)
    expect(wrapper.find('.data').text()).toContain('alpha')
    expect(wrapper.find('.data').text()).toContain('beta')
    expect(wrapper.find('.refreshing').text()).toBe('false')

    await vi.advanceTimersByTimeAsync(10)
    await flushPromises()

    expect(loader).toHaveBeenCalledTimes(3)
    expect(wrapper.find('.error').text()).toBe('boom')
    wrapper.unmount()
  })

  it('reports an initial load failure', async () => {
    const loader = vi.fn(async () => {
      throw new Error('initial failure')
    })

    const Component = defineComponent({
      setup() {
        return useAsyncData(loader)
      },
      template: `
        <div>
          <span class="loading">{{ String(isLoading) }}</span>
          <span class="error">{{ error ?? '' }}</span>
        </div>
      `,
    })

    const wrapper = mount(Component)
    await flushPromises()

    expect(loader).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.loading').text()).toBe('false')
    expect(wrapper.find('.error').text()).toBe('initial failure')
    wrapper.unmount()
  })

  it('accepts a poll interval getter function', async () => {
    const loader = vi.fn(async () => ['value'])
    const Component = defineComponent({
      setup() {
        return useAsyncData(loader, {
          pollIntervalMs: () => 0,
        })
      },
      template: `
        <div>
          <span class="data">{{ JSON.stringify(data) }}</span>
        </div>
      `,
    })

    const wrapper = mount(Component)
    await flushPromises()

    expect(loader).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.data').text()).toContain('value')
    wrapper.unmount()
  })
})
