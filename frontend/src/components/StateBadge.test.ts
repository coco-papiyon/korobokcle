import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import StateBadge from './StateBadge.vue'

describe('StateBadge', () => {
  it('chooses tone classes based on state', () => {
    expect(mount(StateBadge, { props: { state: 'failed' } }).classes()).toContain('state-badge--danger')
    expect(mount(StateBadge, { props: { state: 'rejected' } }).classes()).toContain('state-badge--danger')
    expect(mount(StateBadge, { props: { state: 'waiting' } }).classes()).toContain('state-badge--warning')
    expect(mount(StateBadge, { props: { state: 'completed' } }).classes()).toContain('state-badge--success')
    expect(mount(StateBadge, { props: { state: 'review_ready' } }).classes()).toContain('state-badge--success')
    expect(mount(StateBadge, { props: { state: 'enabled' } }).classes()).toContain('state-badge--success')
    expect(mount(StateBadge, { props: { state: 'draft' } }).classes()).toContain('state-badge--neutral')
    expect(mount(StateBadge, { props: { state: 'waiting_final_approval' } }).text()).toContain('最終承認待ち')
  })
})
