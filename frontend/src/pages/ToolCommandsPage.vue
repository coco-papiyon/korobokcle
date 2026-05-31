<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchToolCommands, saveToolCommands } from '@/lib/api'
import { UNKNOWN_ERROR_MESSAGE } from '@/lib/ui-text'
import type { ToolCommand } from '@/types'

type ToolCommandForm = ToolCommand & {
  id: string
}

const { data, isLoading, error, reload } = useAsyncData(fetchToolCommands)
const commands = ref<ToolCommandForm[]>([])
const selectedID = ref('')
const nextCommandID = ref(1)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)

watch(
  data,
  (value) => {
    commands.value = (value ?? []).map(toForm)
    if (!selectedID.value || !commands.value.some((command) => command.id === selectedID.value)) {
      selectedID.value = commands.value[0]?.id ?? ''
    }
  },
  { immediate: true },
)

const selectedCommand = computed(() => commands.value.find((command) => command.id === selectedID.value) ?? null)

function toForm(command: ToolCommand): ToolCommandForm {
  return {
    id: `tool-command-${nextCommandID.value++}`,
    ...command,
  }
}

function selectCommand(id: string) {
  selectedID.value = id
}

function addCommand() {
  const suffix = nextCommandID.value
  const command: ToolCommandForm = {
    id: `tool-command-${nextCommandID.value++}`,
    name: `tool-${suffix}`,
    command: 'npm run dev',
    resident: true,
  }
  commands.value = [...commands.value, command]
  selectedID.value = command.id
  saveState.value = 'idle'
  saveError.value = null
}

function removeSelectedCommand() {
  if (!selectedCommand.value) {
    return
  }
  commands.value = commands.value.filter((command) => command.id !== selectedCommand.value?.id)
  selectedID.value = commands.value[0]?.id ?? ''
  saveState.value = 'idle'
  saveError.value = null
}

function validateCommands(value: ToolCommandForm[]): string | null {
  const seen = new Set<string>()
  for (const [index, command] of value.entries()) {
    const name = command.name.trim()
    if (!name) {
      return `toolCommand[${index}].name は必須です。`
    }
    if (seen.has(name)) {
      return `toolCommand[${index}].name は重複できません: ${name}`
    }
    seen.add(name)
    if (!command.command.trim()) {
      return `toolCommand[${index}].command は必須です。`
    }
  }
  return null
}

async function persistCommands() {
  const validationError = validateCommands(commands.value)
  if (validationError) {
    saveState.value = 'error'
    saveError.value = validationError
    return
  }

  const selectedIndex = Math.max(0, commands.value.findIndex((command) => command.id === selectedID.value))
  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveToolCommands(commands.value.map((command) => ({
      name: command.name.trim(),
      command: command.command.trim(),
      resident: command.resident,
    })))
    commands.value = saved.map(toForm)
    selectedID.value = commands.value[selectedIndex]?.id ?? commands.value[0]?.id ?? ''
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
    title="ツールコマンド"
    description="動作確認用のコマンドと、常駐/非常駐の種別を設定します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="watch-layout">
        <aside class="panel rule-list">
          <div class="rule-list__header">
            <div>
              <h2>コマンド一覧</h2>
              <p class="text-muted">watch rule から選択する動作確認コマンドです。</p>
            </div>
            <button class="button button-primary" type="button" @click="addCommand">コマンドを追加</button>
          </div>

          <div class="stack-sm">
            <button
              v-for="command in commands"
              :key="command.id"
              class="rule-item"
              :class="{ 'rule-item--active': selectedID === command.id }"
              type="button"
              @click="selectCommand(command.id)"
            >
              <div class="rule-item__head">
                <strong>{{ command.name }}</strong>
                <span class="text-muted">{{ command.resident ? '常駐' : '単発' }}</span>
              </div>
              <p class="text-muted">{{ command.command }}</p>
            </button>
          </div>
        </aside>

        <section class="panel rule-editor">
          <div class="rule-editor__header">
            <div>
              <h2>コマンド編集</h2>
              <p class="text-muted">`config/tool-commands.yaml` に保存します。</p>
            </div>
            <div class="button-row">
              <button class="button button-secondary" type="button" :disabled="!selectedCommand" @click="removeSelectedCommand">
                削除
              </button>
              <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistCommands">
                {{ saveState === 'saving' ? '保存中...' : 'コマンドを保存' }}
              </button>
            </div>
          </div>

          <template v-if="selectedCommand">
            <div class="form-grid">
              <label class="field field-full">
                <span class="field__label">コマンド名</span>
                <input v-model="selectedCommand.name" class="field__control" type="text" />
              </label>

              <label class="field field-full">
                <span class="field__label">コマンド</span>
                <textarea
                  v-model="selectedCommand.command"
                  class="field__control field__control--textarea"
                  rows="5"
                  spellcheck="false"
                  placeholder="npm run dev"
                />
              </label>

              <label class="field field-checkbox field-full">
                <input v-model="selectedCommand.resident" type="checkbox" />
                <span>常駐コマンド</span>
              </label>
            </div>

            <div v-if="saveState === 'saved'" class="notice notice-success">tool-commands.yaml を更新しました。</div>
            <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
          </template>

          <div v-else class="notice">コマンドを追加するか、左側の一覧から command を選択してください。</div>
        </section>
      </section>
    </AsyncState>
  </AppShell>
</template>
