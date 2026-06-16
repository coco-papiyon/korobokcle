import { describe, expect, it } from 'vitest'

import { router } from './router'

describe('router', () => {
  it('exposes the expected routes', () => {
    expect(router.getRoutes().map((route) => route.path)).toEqual(
      expect.arrayContaining([
        '/',
        '/guide',
        '/improvements',
        '/jobs/:id',
        '/settings',
        '/settings/workers',
        '/settings/test-profiles',
        '/settings/tool-commands',
        '/settings/watch-rules',
        '/settings/skillsets',
      ]),
    )
  })
})
