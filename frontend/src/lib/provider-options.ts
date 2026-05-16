import type { ProviderSpec } from '@/types'

export type SelectOption = {
  value: string
  label: string
}

export function providerOptions(providers: ProviderSpec[]): SelectOption[] {
  return providers.map((provider) => ({
    value: provider.name,
    label: displayName(provider.name),
  }))
}

export function watchRuleProviderOptions(providers: ProviderSpec[]): SelectOption[] {
  return [
    { value: '', label: 'Use setting' },
    ...providerOptions(providers),
  ]
}

export function modelOptionsForProvider(
  providers: ProviderSpec[],
  providerName: string,
  currentModel = '',
  defaultLabel = 'Default',
): SelectOption[] {
  const provider = providers.find((item) => item.name === providerName)
  const options: SelectOption[] = [{ value: '', label: defaultLabel }]

  for (const model of provider?.models ?? []) {
    const trimmed = model.trim()
    if (!trimmed || options.some((option) => option.value === trimmed)) {
      continue
    }
    options.push({ value: trimmed, label: trimmed })
  }

  const trimmedCurrent = currentModel.trim()
  if (trimmedCurrent && !options.some((option) => option.value === trimmedCurrent)) {
    options.push({ value: trimmedCurrent, label: trimmedCurrent })
  }

  return options
}

function displayName(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) {
    return value
  }
  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1)
}
