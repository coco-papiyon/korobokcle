import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import App from './App.vue'

describe('App', () => {
  it('renders the router outlet', () => {
    const wrapper = mount(App, {
      global: {
        stubs: {
          RouterView: {
            template: '<div class="router-view">route</div>',
          },
        },
      },
    })

    expect(wrapper.find('.router-view').exists()).toBe(true)
  })
})
