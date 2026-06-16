import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import PanelCard from './PanelCard.vue'

describe('PanelCard', () => {
  it('renders the title, optional description, and slot content', () => {
    const withDescription = mount(PanelCard, {
      props: {
        title: 'Overview',
        description: 'Details',
      },
      slots: {
        default: '<p class="panel-content">content</p>',
      },
    })

    expect(withDescription.text()).toContain('Overview')
    expect(withDescription.text()).toContain('Details')
    expect(withDescription.find('.panel-content').exists()).toBe(true)

    const withoutDescription = mount(PanelCard, {
      props: {
        title: 'Overview',
      },
    })
    expect(withoutDescription.text()).toContain('Overview')
    expect(withoutDescription.text()).not.toContain('Details')
  })
})
