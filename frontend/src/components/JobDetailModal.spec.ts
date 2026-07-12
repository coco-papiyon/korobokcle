import { h, onMounted } from 'vue'
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

  it('shows the source diff button next to the close button when available', async () => {
    const showChat = vi.fn()
    const openSourceDiff = vi.fn()
    const openEditView = vi.fn()
    const showLogs = vi.fn()
    const openRuntimeView = vi.fn()
    const wrapper = mount(JobDetailModal, {
      props: {
        jobId: 'job-1',
        refreshKey: 0,
      },
      global: {
        stubs: {
          JobDetailPanel: {
            emits: ['source-diff-availability', 'artifact-edit-availability', 'runtime-availability'],
            setup(_, { expose, emit }) {
              expose({
                openChatView: showChat,
                openSourceDiff,
                openEditView,
                openLogsView: showLogs,
                openRuntimeView,
              })
              onMounted(() => {
                emit('source-diff-availability', true)
                emit('artifact-edit-availability', true)
                emit('runtime-availability', true)
              })
              return () => h('div', { class: 'job-detail-panel-stub' })
            },
          },
        },
      },
    })
    await flushPromises()

    const buttons = wrapper.findAll('.modal-dialog__header button')
    expect(buttons.map((button) => button.text())).toEqual(['チャット', '差分確認', '編集', '動作確認', 'ログ', '閉じる'])
    expect(showChat).toHaveBeenCalledTimes(1)

    expect(buttons[0].classes()).toContain('modal-dialog__action--active')
    await buttons[1].trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.modal-dialog__header button')[1].classes()).toContain('modal-dialog__action--active')
    expect(openSourceDiff).toHaveBeenCalledTimes(1)
    await buttons[2].trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.modal-dialog__header button')[2].classes()).toContain('modal-dialog__action--active')
    expect(openEditView).toHaveBeenCalledTimes(1)
    await buttons[3].trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.modal-dialog__header button')[3].classes()).toContain('modal-dialog__action--active')
    expect(openRuntimeView).toHaveBeenCalledTimes(1)
    await buttons[4].trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.modal-dialog__header button')[4].classes()).toContain('modal-dialog__action--active')
    expect(showLogs).toHaveBeenCalledTimes(1)
    await buttons[0].trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.modal-dialog__header button')[0].classes()).toContain('modal-dialog__action--active')
    expect(showChat).toHaveBeenCalledTimes(2)

    wrapper.unmount()
  })
})
