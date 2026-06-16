import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DataTable from './DataTable.vue'

describe('DataTable', () => {
  it('renders columns and body slot', () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: ['A', 'B'],
      },
      slots: {
        default: '<tr><td>1</td><td>2</td></tr>',
      },
    })

    expect(wrapper.findAll('th').map((cell) => cell.text())).toEqual(['A', 'B'])
    expect(wrapper.find('tbody tr').text()).toContain('1')
    expect(wrapper.find('tbody tr').text()).toContain('2')
  })
})
