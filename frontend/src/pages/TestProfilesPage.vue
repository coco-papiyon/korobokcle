<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchTestProfiles, saveTestProfiles } from '@/lib/api'
import { UNKNOWN_ERROR_MESSAGE } from '@/lib/ui-text'
import type { TestProfile } from '@/types'

type TestProfileForm = TestProfile & {
  id: string
  commandsText: string
}

const { data, isLoading, error, reload } = useAsyncData(fetchTestProfiles)
const profiles = ref<TestProfileForm[]>([])
const selectedID = ref('')
const nextProfileID = ref(1)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)

watch(
  data,
  (value) => {
    profiles.value = (value ?? []).map(toForm)
    if (!selectedID.value || !profiles.value.some((profile) => profile.id === selectedID.value)) {
      selectedID.value = profiles.value[0]?.id ?? ''
    }
  },
  { immediate: true },
)

const selectedProfile = computed(() => profiles.value.find((profile) => profile.id === selectedID.value) ?? null)

function toForm(profile: TestProfile): TestProfileForm {
  return {
    id: `profile-${nextProfileID.value++}`,
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

function selectProfile(id: string) {
  selectedID.value = id
}

function addProfile() {
  const suffix = nextProfileID.value
  const profile: TestProfileForm = {
    id: `profile-${nextProfileID.value++}`,
    name: `profile-${suffix}`,
    commands: ['go test ./...'],
    commandsText: 'go test ./...',
  }
  profiles.value = [...profiles.value, profile]
  selectedID.value = profile.id
  saveState.value = 'idle'
  saveError.value = null
}

function removeSelectedProfile() {
  if (!selectedProfile.value) {
    return
  }
  profiles.value = profiles.value.filter((profile) => profile.id !== selectedProfile.value?.id)
  selectedID.value = profiles.value[0]?.id ?? ''
  saveState.value = 'idle'
  saveError.value = null
}

function validateProfiles(value: TestProfileForm[]): string | null {
  const seen = new Set<string>()
  for (const [index, profile] of value.entries()) {
    const name = profile.name.trim()
    if (!name) {
      return `profile[${index}].name は必須です。`
    }
    if (seen.has(name)) {
      return `profile[${index}].name は重複できません: ${name}`
    }
    seen.add(name)
    if (splitCommands(profile.commandsText).length === 0) {
      return `profile[${index}].commands には 1 つ以上のコマンドが必要です。`
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
    profiles.value.findIndex((profile) => profile.id === selectedID.value),
  )
  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveTestProfiles(profiles.value.map(fromForm))
    profiles.value = saved.map(toForm)
    selectedID.value = profiles.value[selectedIndex]?.id ?? profiles.value[0]?.id ?? ''
    saveState.value = 'saved'
    await reload()
  } catch (err) {
    saveState.value = 'error'
    saveError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}
</script>

<template>
  <AppShell
    title="テストプロファイル"
    description="test-profile のコマンド群を複数行テキストで編集します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="watch-layout">
        <aside class="panel rule-list">
          <div class="rule-list__header">
            <div>
              <h2>プロファイル一覧</h2>
              <p class="text-muted">1 profile につき 1 つのコマンド群を管理します。</p>
            </div>
            <button class="button button-primary" type="button" @click="addProfile">プロファイルを追加</button>
          </div>

          <div class="stack-sm">
            <button
              v-for="profile in profiles"
              :key="profile.id"
              class="rule-item"
              :class="{ 'rule-item--active': selectedID === profile.id }"
              type="button"
              @click="selectProfile(profile.id)"
            >
              <div class="rule-item__head">
                <strong>{{ profile.name }}</strong>
                <span class="text-muted">{{ splitCommands(profile.commandsText).length }} コマンド</span>
              </div>
              <p class="text-muted">{{ previewCommands(profile.commandsText) || 'コマンドなし' }}</p>
            </button>
          </div>
        </aside>

        <section class="panel rule-editor">
          <div class="rule-editor__header">
            <div>
              <h2>プロファイル編集</h2>
              <p class="text-muted">profile 名と commands を編集して `config/test-profiles.yaml` に保存します。</p>
            </div>
            <div class="button-row">
              <button class="button button-secondary" type="button" :disabled="!selectedProfile" @click="removeSelectedProfile">
                削除
              </button>
              <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistProfiles">
                {{ saveState === 'saving' ? '保存中...' : 'プロファイルを保存' }}
              </button>
            </div>
          </div>

          <template v-if="selectedProfile">
            <div class="form-grid">
              <label class="field field-full">
                <span class="field__label">プロファイル名</span>
                <input v-model="selectedProfile.name" class="field__control" type="text" />
              </label>

              <label class="field field-full">
                <span class="field__label">コマンド</span>
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

          <div v-else class="notice">プロファイルを追加するか、左側の一覧から profile を選択してください。</div>
        </section>
      </section>
    </AsyncState>
  </AppShell>
</template>
