<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchTestProfiles, saveTestProfiles } from '@/lib/api'
import type { TestProfile } from '@/types'

type TestProfileForm = TestProfile & {
  commandsText: string
}

const { data, isLoading, error, reload } = useAsyncData(fetchTestProfiles)
const profiles = ref<TestProfileForm[]>([])
const selectedName = ref('')
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)

watch(
  data,
  (value) => {
    profiles.value = (value ?? []).map(toForm)
    if (!selectedName.value || !profiles.value.some((profile) => profile.name === selectedName.value)) {
      selectedName.value = profiles.value[0]?.name ?? ''
    }
  },
  { immediate: true },
)

const selectedProfile = computed(() => profiles.value.find((profile) => profile.name === selectedName.value) ?? null)

function toForm(profile: TestProfile): TestProfileForm {
  return {
    ...profile,
    commands: [...(profile.commands ?? [])],
    commandsText: (profile.commands ?? []).join('\n'),
  }
}

function fromForm(profile: TestProfileForm): TestProfile {
  return {
    name: profile.name.trim(),
    commands: splitCommands(profile.commandsText),
  }
}

function splitCommands(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

function previewCommands(value: string): string {
  return splitCommands(value).slice(0, 2).join(' / ')
}

function selectProfile(name: string) {
  selectedName.value = name
}

function addProfile() {
  const suffix = profiles.value.length + 1
  const profile: TestProfileForm = {
    name: `profile-${suffix}`,
    commands: ['go test ./...'],
    commandsText: 'go test ./...',
  }
  profiles.value = [...profiles.value, profile]
  selectedName.value = profile.name
  saveState.value = 'idle'
  saveError.value = null
}

function removeSelectedProfile() {
  if (!selectedProfile.value) {
    return
  }
  profiles.value = profiles.value.filter((profile) => profile.name !== selectedProfile.value?.name)
  selectedName.value = profiles.value[0]?.name ?? ''
  saveState.value = 'idle'
  saveError.value = null
}

function validateProfiles(value: TestProfileForm[]): string | null {
  const seen = new Set<string>()
  for (const [index, profile] of value.entries()) {
    const name = profile.name.trim()
    if (!name) {
      return `profile[${index}].name is required.`
    }
    if (seen.has(name)) {
      return `profile[${index}].name must be unique: ${name}`
    }
    seen.add(name)
    if (splitCommands(profile.commandsText).length === 0) {
      return `profile[${index}].commands must include at least one command.`
    }
  }
  return null
}

async function persistProfiles() {
  const validationError = validateProfiles(profiles.value)
  if (validationError) {
    saveState.value = 'error'
    saveError.value = validationError
    return
  }

  const selectedIndex = Math.max(
    0,
    profiles.value.findIndex((profile) => profile.name === selectedName.value),
  )
  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveTestProfiles(profiles.value.map(fromForm))
    profiles.value = saved.map(toForm)
    selectedName.value = profiles.value[selectedIndex]?.name ?? profiles.value[0]?.name ?? ''
    saveState.value = 'saved'
    await reload()
  } catch (err) {
    saveState.value = 'error'
    saveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}
</script>

<template>
  <AppShell
    title="Test Profiles"
    description="test-profile のコマンド群を複数行テキストで編集します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="watch-layout">
        <aside class="panel rule-list">
          <div class="rule-list__header">
            <div>
              <h2>Profile List</h2>
              <p class="text-muted">1 profile につき 1 つのコマンド群を管理します。</p>
            </div>
            <button class="button button-primary" type="button" @click="addProfile">Add Profile</button>
          </div>

          <div class="stack-sm">
            <button
              v-for="(profile, index) in profiles"
              :key="`${index}-${profile.name}`"
              class="rule-item"
              :class="{ 'rule-item--active': selectedName === profile.name }"
              type="button"
              @click="selectProfile(profile.name)"
            >
              <div class="rule-item__head">
                <strong>{{ profile.name }}</strong>
                <span class="text-muted">{{ splitCommands(profile.commandsText).length }} commands</span>
              </div>
              <p class="text-muted">{{ previewCommands(profile.commandsText) || 'no commands' }}</p>
            </button>
          </div>
        </aside>

        <section class="panel rule-editor">
          <div class="rule-editor__header">
            <div>
              <h2>Profile Editor</h2>
              <p class="text-muted">profile 名と commands を編集して `config/test-profiles.yaml` に保存します。</p>
            </div>
            <div class="button-row">
              <button class="button button-secondary" type="button" :disabled="!selectedProfile" @click="removeSelectedProfile">
                Delete
              </button>
              <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistProfiles">
                {{ saveState === 'saving' ? 'Saving...' : 'Save Profiles' }}
              </button>
            </div>
          </div>

          <template v-if="selectedProfile">
            <div class="form-grid">
              <label class="field field-full">
                <span class="field__label">Profile Name</span>
                <input v-model="selectedProfile.name" class="field__control" type="text" />
              </label>

              <label class="field field-full">
                <span class="field__label">Commands</span>
                <textarea
                  v-model="selectedProfile.commandsText"
                  class="field__control field__control--textarea"
                  rows="12"
                  spellcheck="false"
                  placeholder="go test ./...&#10;go test ./internal/..."
                />
                <p class="text-muted">1 行を 1 コマンドとして保存します。空行は無視され、前後の空白は削除されます。</p>
              </label>
            </div>

            <div v-if="saveState === 'saved'" class="notice notice-success">test-profiles.yaml を更新しました。</div>
            <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
          </template>

          <div v-else class="notice">Add Profile を押すか、左側の一覧から profile を選択してください。</div>
        </section>
      </section>
    </AsyncState>
  </AppShell>
</template>
