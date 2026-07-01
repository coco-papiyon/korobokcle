import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, vi } from 'vitest'
import JobDetailModal from './JobDetailModal.vue'

async function flushPromises() {
  await Promise.resolve()
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}

describe('JobDetailModal', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('closes from the button, backdrop, and Escape key', async () => {
    const wrapper = mount(JobDetailModal, {
      props: {
        jobId: 'job-1',
        refreshKey: 0,
      },
      global: {
        stubs: {
          JobDetailPanel: {
            template: '<div class="job-detail-panel-stub" />',
          },
        },
      },
    })
    await flushPromises()

    expect(document.body.classList.contains('modal-open')).toBe(true)
    expect(wrapper.get('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.get('.modal-dialog__close').text()).toBe('閉じる')

    await wrapper.get('.modal-dialog__close').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)

    await wrapper.find('.modal-overlay').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(2)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(wrapper.emitted('close')).toHaveLength(3)

    wrapper.unmount()
    expect(document.body.classList.contains('modal-open')).toBe(false)
  })
})
