import { describe, expect, it } from 'vitest'

import { modelOptionsForProvider, providerOptions, watchRuleProviderOptions } from './provider-options'

describe('provider options helpers', () => {
  it('formats provider and watch rule options', () => {
    expect(
      providerOptions([
        { name: 'mock', models: [] },
        { name: '  copilot', models: [] },
        { name: '   ', models: [] },
      ]),
    ).toEqual([
      { value: 'mock', label: 'Mock' },
      { value: '  copilot', label: 'Copilot' },
      { value: '   ', label: '   ' },
    ])

    expect(watchRuleProviderOptions([{ name: 'mock', models: [] }])).toEqual([
      { value: '', label: '設定を使用' },
      { value: 'mock', label: 'Mock' },
    ])
  })

  it('builds model options with deduped and current values', () => {
    expect(
      modelOptionsForProvider(
        [
          {
            name: 'copilot',
            models: ['gpt-4.1', 'gpt-4.1', '  o4-mini  ', ''],
          },
        ],
        'copilot',
        'custom-model',
      ),
    ).toEqual([
      { value: '', label: '既定' },
      { value: 'gpt-4.1', label: 'gpt-4.1' },
      { value: 'o4-mini', label: 'o4-mini' },
      { value: 'custom-model', label: 'custom-model' },
    ])

    expect(modelOptionsForProvider([], 'missing')).toEqual([{ value: '', label: '既定' }])
  })
})
