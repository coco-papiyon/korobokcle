<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import PanelCard from '@/components/PanelCard.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, fetchNotificationConfig, saveAppConfig, saveNotificationConfig } from '@/lib/api'
import { modelOptionsForProvider, providerOptions } from '@/lib/provider-options'
import type { NotificationChannel, ProviderSpec } from '@/types'

const { data, isLoading, error, reload } = useAsyncData(fetchAppConfig)
const { data: notificationData, reload: reloadNotificationData } = useAsyncData(fetchNotificationConfig)
const provider = ref('mock')
const model = ref('')
const pollInterval = ref(120)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)
const providerCatalog = ref<ProviderSpec[]>([])
const notificationChannels = ref<NotificationChannel[]>([])
const notificationSaveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const notificationSaveError = ref<string | null>(null)

const notificationEventOptions = [
  { value: 'design_ready', label: 'Design Ready' },
  { value: 'waiting_design_approval', label: 'Waiting Design Approval' },
  { value: 'implementation_ready', label: 'Implementation Ready' },
  { value: 'waiting_final_approval', label: 'Waiting Final Approval' },
  { value: 'review_ready', label: 'Review Ready' },
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
    pollInterval.value = config?.pollInterval ?? 120
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
  const interval = Number(pollInterval.value)
  if (!Number.isInteger(interval) || interval < 1 || interval > 86400) {
    saveState.value = 'error'
    saveError.value = 'Poll interval must be a whole number between 1 and 86400 seconds.'
    return
  }

  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveAppConfig({ provider: provider.value, model: model.value, pollInterval: interval })
    provider.value = saved.provider
    model.value = saved.model
    pollInterval.value = saved.pollInterval
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
</script>

<template>
  <AppShell title="Settings" description="AI provider などアプリ全体の挙動を設定します。">
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="hero-grid">
        <PanelCard title="Application Settings" description="起動後に変更可能なアプリ設定を編集します。" />
        <PanelCard title="Current Defaults" description="provider / model / pollInterval の現在値を確認できます。" />
      </section>

      <section class="panel stack-md">
        <div class="rule-editor__header">
          <div>
            <h2>Application Settings</h2>
            <p class="text-muted">provider、model、pollInterval を画面から変更できます。</p>
          </div>
          <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistConfig">
            {{ saveState === 'saving' ? 'Saving...' : 'Save Settings' }}
          </button>
        </div>

        <label class="field">
          <span class="field__label">Provider</span>
          <select v-model="provider" class="field__control">
            <option v-for="option in providerOptions(providerCatalog)" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>

        <label class="field">
          <span class="field__label">Model</span>
          <select v-model="model" class="field__control">
            <option v-for="option in availableModelOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>

        <label class="field">
          <span class="field__label">Poll Interval (seconds)</span>
          <input
            v-model.number="pollInterval"
            class="field__control"
            type="number"
            min="1"
            max="86400"
            step="1"
          />
          <p class="text-muted">Whole seconds only. The watcher uses the updated value on the next poll cycle.</p>
        </label>

        <div v-if="saveState === 'saved'" class="notice notice-success">app.yaml を更新しました。</div>
        <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
      </section>

      <section class="panel stack-md">
        <div class="rule-editor__header">
          <div>
            <h2>Notification Settings</h2>
            <p class="text-muted">通知チャネルごとに、どのタイミングで通知するかを切り替えます。</p>
          </div>
          <button
            class="button button-primary"
            type="button"
            :disabled="notificationSaveState === 'saving'"
            @click="persistNotificationConfig"
          >
            {{ notificationSaveState === 'saving' ? 'Saving...' : 'Save Notifications' }}
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
