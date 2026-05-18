<script setup lang="ts">
import { ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, saveAppConfig } from '@/lib/api'
import type { MonitoredRepository } from '@/types'

const { data, isLoading, error, reload } = useAsyncData(fetchAppConfig)
const monitoredRepositories = ref<MonitoredRepository[]>([])
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)

watch(
  data,
  (config) => {
    monitoredRepositories.value = (config?.monitoredRepositories ?? []).map((entry) => ({
      repository: entry.repository ?? '',
      workers: Math.max(1, Number(entry.workers) || 1),
    }))
  },
  { immediate: true },
)

function addMonitoredRepository() {
  monitoredRepositories.value = [...monitoredRepositories.value, { repository: '', workers: 1 }]
}

function removeMonitoredRepository(index: number) {
  monitoredRepositories.value = monitoredRepositories.value.filter((_, currentIndex) => currentIndex !== index)
}

function normalizeMonitoredRepositories(values: MonitoredRepository[]) {
  return values
    .map((entry) => ({
      repository: entry.repository.trim(),
      workers: Math.floor(Number(entry.workers)),
    }))
    .filter((entry) => entry.repository.length > 0)
    .map((entry) => ({
      repository: entry.repository,
      workers: Number.isInteger(entry.workers) && entry.workers >= 1 ? entry.workers : 1,
    }))
    .filter((entry, index, items) => items.findIndex((candidate) => candidate.repository === entry.repository) === index)
}

async function persistConfig() {
  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveAppConfig({
      monitoredRepositories: normalizeMonitoredRepositories(monitoredRepositories.value),
    })
    monitoredRepositories.value = (saved.monitoredRepositories ?? []).map((entry) => ({
      repository: entry.repository ?? '',
      workers: Math.max(1, Number(entry.workers) || 1),
    }))
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
    title="Workers"
    description="監視対象リポジトリと、各リポジトリに割り当てるワーカー数を設定します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="panel stack-md">
        <div class="rule-editor__header">
          <div>
            <h2>Worker Settings</h2>
            <p class="text-muted">1 行につき 1 リポジトリを追加し、1 以上のワーカー数を指定します。</p>
          </div>
          <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistConfig">
            {{ saveState === 'saving' ? 'Saving...' : 'Save Workers' }}
          </button>
        </div>

        <div class="field">
          <div class="rule-editor__header">
            <span class="field__label">Monitored Repositories</span>
            <button class="button button-secondary" type="button" @click="addMonitoredRepository">Add Repository</button>
          </div>
          <div class="stack-sm">
            <div v-for="(entry, index) in monitoredRepositories" :key="`${index}-${entry.repository}`" class="form-grid">
              <label class="field field-full">
                <span class="field__label">Repository</span>
                <input v-model="entry.repository" class="field__control" type="text" placeholder="owner/repository" />
              </label>
              <label class="field">
                <span class="field__label">Workers</span>
                <input v-model.number="entry.workers" class="field__control" type="number" min="1" step="1" />
              </label>
              <button class="button button-secondary" type="button" @click="removeMonitoredRepository(index)">Remove</button>
            </div>
          </div>
          <p class="text-muted">ワーカー数は 1 以上の整数です。watch rules 側では、ここで登録したリポジトリのみ選択できます。</p>
        </div>

        <div v-if="saveState === 'saved'" class="notice notice-success">workers を更新しました。</div>
        <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
      </section>
    </AsyncState>
  </AppShell>
</template>
