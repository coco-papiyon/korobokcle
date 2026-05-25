<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, fetchNotificationConfig, saveAppConfig, saveNotificationConfig } from '@/lib/api'
import { modelOptionsForProvider, providerOptions } from '@/lib/provider-options'
import type { NotificationChannel, ProviderSpec } from '@/types'

const { data, isLoading, error, reload } = useAsyncData(fetchAppConfig)
const { data: notificationData, reload: reloadNotificationData } = useAsyncData(fetchNotificationConfig)
const provider = ref('mock')
const model = ref('')
const copilotAllowToolsText = ref('')
const pollInterval = ref('120')
const screenRefreshInterval = ref('5')
const shutdownTimeout = ref('10')
const prTitleTemplate = ref('')
const branchTemplate = ref('')
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)
const providerCatalog = ref<ProviderSpec[]>([])
const notificationChannels = ref<NotificationChannel[]>([])
const notificationSaveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const notificationSaveError = ref<string | null>(null)
const templateVariables = ['{{issue_number}}', '{{issue_title}}', '{{repository}}'] as const

const notificationEventOptions = [
  { value: 'waiting_design_approval', label: 'Waiting Design Approval' },
  { value: 'waiting_final_approval', label: 'Waiting Final Approval' },
  { value: 'review_completed', label: 'Review Completed' },
  { value: 'pr_created', label: 'PR Created' },
  { value: 'failed', label: 'Any Failure' },
] as const

const availableModelOptions = computed(() => {
  return modelOptionsForProvider(providerCatalog.value, provider.value, model.value, 'Default')
})

watch(
  data,
  (config) => {
    provider.value = config?.provider ?? 'mock'
    model.value = config?.model ?? ''
    copilotAllowToolsText.value = (config?.copilotAllowTools ?? []).join('\n')
    pollInterval.value = String(config?.pollInterval ?? 120)
    screenRefreshInterval.value = String(config?.screenRefreshInterval ?? 5)
    shutdownTimeout.value = String(config?.shutdownTimeout ?? 10)
    prTitleTemplate.value = config?.prTitleTemplate ?? ''
    branchTemplate.value = config?.branchTemplate ?? ''
    providerCatalog.value = config?.providers ?? []
  },
  { immediate: true },
)

watch(
  notificationData,
  (config) => {
    notificationChannels.value = (config?.channels ?? []).map((channel) => ({
      ...channel,
      events: [...channel.events],
    }))
  },
  { immediate: true },
)

async function persistConfig() {
  const parsedPollInterval = parseIntegerField(pollInterval.value, 'Git polling interval')
  if (parsedPollInterval === null) {
    return
  }
  const parsedScreenRefreshInterval = parseIntegerField(screenRefreshInterval.value, 'Screen refresh interval')
  if (parsedScreenRefreshInterval === null) {
    return
  }
  const parsedShutdownTimeout = parseIntegerField(shutdownTimeout.value, 'Shutdown timeout')
  if (parsedShutdownTimeout === null) {
    return
  }

  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveAppConfig({
      provider: provider.value,
      model: model.value,
      copilotAllowTools: parseCopilotAllowTools(copilotAllowToolsText.value),
      pollInterval: parsedPollInterval,
      screenRefreshInterval: parsedScreenRefreshInterval,
      shutdownTimeout: parsedShutdownTimeout,
      prTitleTemplate: prTitleTemplate.value,
      branchTemplate: branchTemplate.value,
    })
    provider.value = saved.provider
    model.value = saved.model
    copilotAllowToolsText.value = (saved.copilotAllowTools ?? []).join('\n')
    pollInterval.value = String(saved.pollInterval)
    screenRefreshInterval.value = String(saved.screenRefreshInterval)
    shutdownTimeout.value = String(saved.shutdownTimeout)
    prTitleTemplate.value = saved.prTitleTemplate
    branchTemplate.value = saved.branchTemplate
    saveState.value = 'saved'
    await reload()
  } catch (err) {
    saveState.value = 'error'
    saveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

function channelHasEvent(channel: NotificationChannel, eventName: string) {
  return channel.events.includes(eventName)
}

function updateChannelEvent(channel: NotificationChannel, eventName: string, enabled: boolean) {
  if (enabled) {
    if (!channel.events.includes(eventName)) {
      channel.events = [...channel.events, eventName]
    }
    return
  }
  channel.events = channel.events.filter((value) => value !== eventName)
}

async function persistNotificationConfig() {
  notificationSaveState.value = 'saving'
  notificationSaveError.value = null
  try {
    const saved = await saveNotificationConfig({
      channels: notificationChannels.value.map((channel) => ({
        ...channel,
        events: [...channel.events],
      })),
    })
    notificationChannels.value = saved.channels.map((channel) => ({ ...channel, events: [...channel.events] }))
    notificationSaveState.value = 'saved'
    await reloadNotificationData()
  } catch (err) {
    notificationSaveState.value = 'error'
    notificationSaveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

function parseCopilotAllowTools(value: string) {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter((item, index, items) => item.length > 0 && items.indexOf(item) === index)
}

function parseIntegerField(value: string, label: string) {
  if (value.trim() === '') {
    saveState.value = 'error'
    saveError.value = `${label} is required.`
    return null
  }
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed < 0) {
    saveState.value = 'error'
    saveError.value = `${label} must be a non-negative whole number.`
    return null
  }
  return parsed
}

</script>

<template>
  <AppShell
    title="アプリケーション設定"
    description="アプリの動作設定を管理します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="panel stack-md">
        <div class="rule-editor__header">
          <div>
            <h2>アプリケーション設定</h2>
            <p class="text-muted">provider、model、Git ポーリング間隔、画面の自動更新間隔、shutdown timeout をここから変更できます。</p>
          </div>
          <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistConfig">
            {{ saveState === 'saving' ? '保存中...' : '設定を保存' }}
          </button>
        </div>

        <div class="settings-list">
          <label class="settings-row">
            <span class="settings-row__label">プロバイダー</span>
            <select v-model="provider" class="field__control settings-row__control" @change="model = ''">
              <option v-for="option in providerOptions(providerCatalog)" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <p class="settings-row__description text-muted">利用する AI プロバイダーを選択します。</p>
          </label>

          <label class="settings-row">
            <span class="settings-row__label">モデル</span>
            <select v-model="model" class="field__control settings-row__control">
              <option v-for="option in availableModelOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <p class="settings-row__description text-muted">選択したプロバイダーで使うモデルを指定します。</p>
          </label>

          <label v-if="provider === 'copilot'" class="settings-row">
            <span class="settings-row__label">Copilot 許可ツール</span>
            <textarea
              v-model="copilotAllowToolsText"
              class="field__control field__control--textarea settings-row__control"
              rows="6"
              spellcheck="false"
            />
            <p class="settings-row__description text-muted">1 行につき 1 ツールを記述します。例: <code>write</code>, <code>shell(go:*)</code>, <code>shell(git:*)</code>。</p>
          </label>

          <label class="settings-row">
            <span class="settings-row__label">Git ポーリング間隔（秒）</span>
            <input
              v-model="pollInterval"
              class="field__control settings-row__control"
              inputmode="numeric"
              min="0"
              step="1"
              type="number"
            />
            <p class="settings-row__description text-muted">0 にすると Git 監視のポーリングを無効にします。</p>
          </label>

          <label class="settings-row">
            <span class="settings-row__label">画面自動更新間隔（秒）</span>
            <input
              v-model="screenRefreshInterval"
              class="field__control settings-row__control"
              inputmode="numeric"
              min="0"
              step="1"
              type="number"
            />
            <p class="settings-row__description text-muted">0 にすると Dashboard と Job Detail の自動更新を止めます。</p>
          </label>

          <label class="settings-row">
            <span class="settings-row__label">Shutdown Timeout（秒）</span>
            <input
              v-model="shutdownTimeout"
              class="field__control settings-row__control"
              inputmode="numeric"
              min="0"
              step="1"
              type="number"
            />
            <p class="settings-row__description text-muted">終了処理を待つ最大秒数です。整数秒で保存されます。</p>
          </label>

          <label class="settings-row">
            <span class="settings-row__label">PR Title Template</span>
            <input v-model="prTitleTemplate" class="field__control settings-row__control" type="text" />
            <p class="settings-row__description text-muted">
              利用可能な変数:
              <template v-for="(variable, index) in templateVariables" :key="variable">
                <code>{{ variable }}</code><span v-if="index < templateVariables.length - 1">, </span>
              </template>
              。
            </p>
          </label>

          <label class="settings-row">
            <span class="settings-row__label">Branch Template</span>
            <input v-model="branchTemplate" class="field__control settings-row__control" type="text" />
            <p class="settings-row__description text-muted">
              利用可能な変数:
              <template v-for="(variable, index) in templateVariables" :key="variable">
                <code>{{ variable }}</code><span v-if="index < templateVariables.length - 1">, </span>
              </template>
              。
            </p>
          </label>
        </div>

        <div v-if="saveState === 'saved'" class="notice notice-success">app.yaml を更新しました。</div>
        <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
      </section>

      <section class="panel stack-md">
        <div class="rule-editor__header">
          <div>
            <h2>関連ページ</h2>
            <p class="text-muted">test-profile の編集は専用ページから行います。</p>
          </div>
          <RouterLink class="button button-primary" to="/settings/test-profiles">Test Profiles を開く</RouterLink>
        </div>
      </section>

      <section class="panel stack-md">
        <div class="rule-editor__header">
          <div>
            <h2>通知設定</h2>
            <p class="text-muted">通知チャネルごとに、どのタイミングで通知するかを切り替えます。</p>
          </div>
          <button
            class="button button-primary"
            type="button"
            :disabled="notificationSaveState === 'saving'"
            @click="persistNotificationConfig"
          >
            {{ notificationSaveState === 'saving' ? '保存中...' : '通知を保存' }}
          </button>
        </div>

        <div v-for="channel in notificationChannels" :key="`${channel.name}-${channel.type}`" class="stack-sm">
          <label class="field field--inline">
            <span class="field__label">{{ channel.name }} ({{ channel.type }})</span>
            <input v-model="channel.enabled" type="checkbox" />
          </label>

          <div class="stack-sm">
            <label
              v-for="option in notificationEventOptions"
              :key="`${channel.name}-${option.value}`"
              class="field field--inline"
            >
              <span class="field__label">{{ option.label }}</span>
              <input
                :checked="channelHasEvent(channel, option.value)"
                type="checkbox"
                :disabled="!channel.enabled"
                @change="updateChannelEvent(channel, option.value, ($event.target as HTMLInputElement).checked)"
              />
            </label>
          </div>
        </div>

        <div v-if="notificationSaveState === 'saved'" class="notice notice-success">notifications.yaml を更新しました。</div>
        <div v-if="notificationSaveState === 'error'" class="notice notice-danger">{{ notificationSaveError }}</div>
      </section>
    </AsyncState>
  </AppShell>
</template>
