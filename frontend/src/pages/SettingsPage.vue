<script setup lang="ts">
import { ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import PanelCard from '@/components/PanelCard.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, saveAppConfig } from '@/lib/api'

const { data, isLoading, error, reload } = useAsyncData(fetchAppConfig)
const provider = ref('mock')
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)

watch(
  data,
  (config) => {
    provider.value = config?.provider ?? 'mock'
  },
  { immediate: true },
)

async function persistConfig() {
  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveAppConfig({ provider: provider.value })
    provider.value = saved.provider
    saveState.value = 'saved'
    await reload()
  } catch (err) {
    saveState.value = 'error'
    saveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}
</script>

<template>
  <AppShell title="Settings" description="AI provider などアプリ全体の挙動を設定します。">
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="hero-grid">
        <PanelCard title="AI Provider" description="設計・実装フローで利用する CLI ベースの AI provider を選択します。" />
        <PanelCard title="Persistence" description="保存すると config/app.yaml が更新され、次回実行から新しい provider が使われます。" />
      </section>

      <section class="panel stack-md">
        <div class="rule-editor__header">
          <div>
            <h2>Application Settings</h2>
            <p class="text-muted">現在は AI provider を切り替えできます。</p>
          </div>
          <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistConfig">
            {{ saveState === 'saving' ? 'Saving...' : 'Save Settings' }}
          </button>
        </div>

        <label class="field">
          <span class="field__label">Provider</span>
          <select v-model="provider" class="field__control">
            <option value="mock">Mock</option>
            <option value="copilot">Copilot</option>
            <option value="codex">Codex</option>
          </select>
        </label>

        <div v-if="saveState === 'saved'" class="notice notice-success">app.yaml を更新しました。</div>
        <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
      </section>
    </AsyncState>
  </AppShell>
</template>
