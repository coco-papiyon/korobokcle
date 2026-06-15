<script setup lang="ts">
import { ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, saveAppConfig } from '@/lib/api'
import { UNKNOWN_ERROR_MESSAGE } from '@/lib/ui-text'
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
      branch: entry.branch ?? '',
      workDir: entry.workDir ?? '',
      implementationWorkers: Math.max(1, Number(entry.implementationWorkers) || 1),
      reviewWorkers: Math.max(1, Number(entry.reviewWorkers) || 1),
      improvementEnabled: Boolean(entry.improvementEnabled),
      improvementBranch: entry.improvementBranch ?? '',
      improvementDir: entry.improvementDir ?? '',
      workerDirs: entry.workerDirs ?? [],
    }))
  },
  { immediate: true },
)

function addMonitoredRepository() {
  monitoredRepositories.value = [
    ...monitoredRepositories.value,
    {
      repository: '',
      branch: '',
      workDir: '',
      implementationWorkers: 1,
      reviewWorkers: 1,
      improvementEnabled: false,
      improvementBranch: '',
      improvementDir: '',
      workerDirs: [],
    },
  ]
}

function removeMonitoredRepository(index: number) {
  monitoredRepositories.value = monitoredRepositories.value.filter((_, currentIndex) => currentIndex !== index)
}

function repositoryWorkDirComponent(repository: string) {
  const trimmed = repository.trim()
  if (!trimmed) {
    return 'owner-repository'
  }

  try {
    const url = new URL(trimmed)
    const path = url.pathname.replace(/^\/+/, '').replace(/\.git$/i, '')
    if (path) {
      return path.replace(/[\\/]/g, '-')
    }
  } catch {
    // Not a URL. Fall through to the generic normalizer.
  }

  return trimmed
    .replace(/^git@[^:]+:/i, '')
    .replace(/^https?:\/\/[^/]+\//i, '')
    .replace(/\.git$/i, '')
    .replace(/[\\/]/g, '-')
    .replace(/[:@?#]/g, '-')
}

function normalizeMonitoredRepositories(values: MonitoredRepository[]) {
  return values
    .map((entry) => ({
      repository: entry.repository.trim(),
      branch: entry.branch.trim(),
      workDir: entry.workDir.trim(),
      implementationWorkers: Math.floor(Number(entry.implementationWorkers)),
      reviewWorkers: Math.floor(Number(entry.reviewWorkers)),
      improvementEnabled: Boolean(entry.improvementEnabled),
      improvementBranch: entry.improvementBranch.trim(),
      improvementDir: entry.improvementDir.trim(),
    }))
    .filter((entry) => entry.repository.length > 0)
    .map((entry) => ({
      repository: entry.repository,
      branch: entry.branch,
      workDir: entry.workDir,
      implementationWorkers:
        Number.isInteger(entry.implementationWorkers) && entry.implementationWorkers >= 1 ? entry.implementationWorkers : 1,
      reviewWorkers: Number.isInteger(entry.reviewWorkers) && entry.reviewWorkers >= 1 ? entry.reviewWorkers : 1,
      improvementEnabled: entry.improvementEnabled,
      improvementBranch: entry.improvementBranch,
      improvementDir: entry.improvementDir,
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
      branch: entry.branch ?? '',
      workDir: entry.workDir ?? '',
      implementationWorkers: Math.max(1, Number(entry.implementationWorkers) || 1),
      reviewWorkers: Math.max(1, Number(entry.reviewWorkers) || 1),
      improvementEnabled: Boolean(entry.improvementEnabled),
      improvementBranch: entry.improvementBranch ?? '',
      improvementDir: entry.improvementDir ?? '',
      workerDirs: entry.workerDirs ?? [],
    }))
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
    title="ワーカー設定"
    description="監視対象リポジトリと、各リポジトリの作業ディレクトリとワーカー数を設定します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="panel stack-md">
        <div class="rule-editor__header">
          <div>
            <h2>ワーカー設定</h2>
            <p class="text-muted">1 行につき 1 リポジトリを追加し、作業ディレクトリと 1 以上のワーカー数を指定します。</p>
          </div>
          <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistConfig">
            {{ saveState === 'saving' ? '保存中...' : 'ワーカー設定を保存' }}
          </button>
        </div>

        <div class="field">
          <div class="rule-editor__header">
            <span class="field__label">監視対象リポジトリ</span>
            <button class="button button-secondary" type="button" @click="addMonitoredRepository">リポジトリを追加</button>
          </div>
          <div class="stack-sm">
            <div v-for="(entry, index) in monitoredRepositories" :key="`${index}-${entry.repository}`" class="form-grid">
              <label class="field field-full">
                <span class="field__label">リポジトリ</span>
                <input v-model="entry.repository" class="field__control" type="text" placeholder="owner/repository" />
              </label>
              <label class="field field-full">
                <span class="field__label">ブランチ</span>
                <input v-model="entry.branch" class="field__control" type="text" placeholder="main" />
              </label>
              <label class="field field-full">
                <span class="field__label">作業ディレクトリ</span>
                <input
                  v-model="entry.workDir"
                  class="field__control"
                  type="text"
                  :placeholder="`source/${repositoryWorkDirComponent(entry.repository)}`"
                />
              </label>
              <label class="field">
                <span class="field__label">実装 worker 数</span>
                <input v-model.number="entry.implementationWorkers" class="field__control" type="number" min="1" step="1" />
              </label>
              <label class="field">
                <span class="field__label">PRレビュー数</span>
                <input v-model.number="entry.reviewWorkers" class="field__control" type="number" min="1" step="1" />
              </label>
              <label class="field field-full">
                <span class="field__label">改善機能</span>
                <label class="checkbox">
                  <input v-model="entry.improvementEnabled" type="checkbox" />
                  <span>このリポジトリで改善機能を有効にする</span>
                </label>
              </label>
              <label class="field field-full">
                <span class="field__label">改善ブランチ名</span>
                <input v-model="entry.improvementBranch" class="field__control" type="text" placeholder="improvement" />
              </label>
              <label class="field field-full">
                <span class="field__label">改善指示ディレクトリ</span>
                <input v-model="entry.improvementDir" class="field__control" type="text" placeholder=".improvement" />
              </label>
              <button class="button button-secondary" type="button" @click="removeMonitoredRepository(index)">削除</button>
            </div>
          </div>
          <p class="text-muted">作業ディレクトリを空にすると既定の `source/&lt;repo&gt;` を使います。`&lt;repo&gt;` は `owner-repository` のようなリポジトリ識別子です。ブランチを空にするとリモートの既定ブランチを使います。実際の作業用 worktree は `source/&lt;repo&gt;-&lt;branch&gt;` になります。改善設定は空欄なら `improvement` / `.improvement` にフォールバックします。監視ルール側では、ここで登録したリポジトリのみ選択できます。`実装 worker 数` は実装系、`PRレビュー数` は PR レビュー系の並列上限です。</p>
        </div>

        <div v-if="saveState === 'saved'" class="notice notice-success">ワーカー設定を更新しました。</div>
        <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
      </section>
    </AsyncState>
  </AppShell>
</template>
