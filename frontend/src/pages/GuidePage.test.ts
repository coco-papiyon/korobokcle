import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import GuidePage from './GuidePage.vue'

describe('GuidePage', () => {
  it('renders the guide markdown and table of contents', () => {
    const wrapper = mount(GuidePage)

    expect(wrapper.text()).toContain('目次')
    expect(wrapper.findAll('.guide-toc__link').map((link) => link.text())).toEqual(
      expect.arrayContaining(['概要', '基本フロー', 'ツールコマンド', '注意事項', 'インストール']),
    )
    expect(wrapper.find('.markdown-content').exists()).toBe(true)
  })
})
