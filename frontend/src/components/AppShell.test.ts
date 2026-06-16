import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppShell from './AppShell.vue'

describe('AppShell', () => {
  it('renders navigation and slot content', () => {
    const wrapper = mount(AppShell, {
      props: {
        title: 'App',
        description: 'Description',
      },
      slots: {
        default: '<section class="body">body</section>',
      },
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
          },
        },
      },
    })

    expect(wrapper.text()).toContain('Description')
    expect(wrapper.find('.body').exists()).toBe(true)
    expect(wrapper.findAll('a').map((link) => link.attributes('href'))).toContain('/guide')
  })
})
