import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AsyncState from './AsyncState.vue'

describe('AsyncState', () => {
  it('renders loading, error, and slot content states', () => {
    const loading = mount(AsyncState, {
      props: {
        isLoading: true,
        error: null,
      },
    })
    expect(loading.text()).toContain('読み込み中')
    expect(loading.text()).toContain('データを取得しています。')

    const error = mount(AsyncState, {
      props: {
        isLoading: false,
        error: 'failure',
      },
    })
    expect(error.text()).toContain('エラー')
    expect(error.text()).toContain('failure')

    const success = mount(AsyncState, {
      props: {
        isLoading: false,
        error: null,
      },
      slots: {
        default: '<span class="content">ready</span>',
      },
    })
    expect(success.find('.content').exists()).toBe(true)
    expect(success.text()).toContain('ready')
  })
})
