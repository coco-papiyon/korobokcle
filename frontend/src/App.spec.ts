import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'

vi.mock('./components/SettingsPanel.vue', () => ({
  default: defineComponent({
    name: 'SettingsPanelStub',
    render() {
      return h('div', 'settings-stub')
    },
  }),
}))

vi.mock('./components/SkillGeneratorPanel.vue', () => ({
  default: defineComponent({
    name: 'SkillGeneratorPanelStub',
    render() {
      return h('div', 'skills-stub')
    },
  }),
}))

vi.mock('./components/JobListPanel.vue', () => ({
  default: defineComponent({
    name: 'JobListPanelStub',
    props: {
      active: {
        type: Boolean,
        required: true,
      },
      selectedJobId: {
        type: String,
        required: true,
      },
    },
    emits: ['select'],
    render() {
      return h('div', { class: 'job-list-stub' }, [
        h('span', `active=${String(this.active)} selected=${this.selectedJobId}`),
        h(
          'button',
          {
            type: 'button',
            onClick: () => this.$emit('select', 'job-1'),
          },
          'select-job',
        ),
      ])
    },
  }),
}))

vi.mock('./components/JobDetailPanel.vue', () => ({
  default: defineComponent({
    name: 'JobDetailPanelStub',
    props: {
      active: {
        type: Boolean,
        required: true,
      },
      jobId: {
        type: String,
        required: true,
      },
      refreshKey: {
        type: Number,
        required: true,
      },
    },
    render() {
      return h('div', { class: 'detail-stub' }, `active=${String(this.active)} job=${this.jobId} key=${this.refreshKey}`)
    },
  }),
}))

import App from './App.vue'

async function flushPromises() {
  await Promise.resolve()
  await nextTick()
  await Promise.resolve()
  await nextTick()
}

describe('App', () => {
  it('passes active state to job panels when tabs change', async () => {
    const wrapper = mount(App)
    await flushPromises()

    expect(wrapper.get('.job-list-stub').text()).toContain('active=true')
    expect(wrapper.get('.detail-stub').text()).toContain('active=false')

    await wrapper.get('.job-list-stub button').trigger('click')
    await flushPromises()

    expect(wrapper.get('.job-list-stub').text()).toContain('active=false')
    expect(wrapper.get('.detail-stub').text()).toContain('active=true')
    expect(wrapper.get('.detail-stub').text()).toContain('job=job-1')
    expect(wrapper.get('.detail-stub').text()).toContain('key=1')

    const jobTab = wrapper.findAll('button[role="tab"]').find((button) => button.text() === 'ジョブ一覧')
    expect(jobTab).toBeTruthy()
    await jobTab!.trigger('click')
    await flushPromises()

    expect(wrapper.get('.job-list-stub').text()).toContain('active=true')
    expect(wrapper.get('.detail-stub').text()).toContain('active=false')
  })
})
