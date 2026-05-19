<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, fetchJobs } from '@/lib/api'
import { formatDateTime } from '@/lib/format'
import type { Job } from '@/types'

function mergeJobs(current: Job[] | null, incoming: Job[]) {
  if (!current) {
    return incoming
  }

  const currentByID = new Map(current.map((job) => [job.id, job]))
  return incoming.map((job) => {
    const existing = currentByID.get(job.id)
    if (!existing) {
      return job
    }
    if (
      existing.type === job.type &&
      existing.repository === job.repository &&
      existing.githubNumber === job.githubNumber &&
      existing.state === job.state &&
      existing.title === job.title &&
      existing.branchName === job.branchName &&
      existing.watchRuleId === job.watchRuleId &&
      existing.deletedAt === job.deletedAt &&
      existing.createdAt === job.createdAt &&
      existing.updatedAt === job.updatedAt
    ) {
      return existing
    }
    return {
      ...existing,
      ...job,
    }
  })
}

const { data: appConfig } = useAsyncData(fetchAppConfig)
const refreshIntervalMs = computed(() => {
  const seconds = appConfig.value?.screenRefreshInterval ?? 0
  return seconds > 0 ? seconds * 1000 : 0
})

const showDeletedOnly = ref(false)

const { data, isLoading, isRefreshing, error, reload } = useAsyncData(() => fetchJobs(showDeletedOnly.value ? 'only' : 'exclude'), {
  pollIntervalMs: refreshIntervalMs,
  mergeData: mergeJobs,
})

watch(showDeletedOnly, () => {
  void reload()
})
</script>

<template>
  <AppShell
    title="korobokcle"
    description="Watch Ruleに一致するGitHub Issue/PRの一覧と自動処理の状況を確認できます。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <p v-if="isRefreshing" class="text-muted">Syncing jobs...</p>
      <div class="button-row">
        <button class="button button-secondary" type="button" @click="showDeletedOnly = !showDeletedOnly">
          {{ showDeletedOnly ? '表示を通常に戻す' : '削除済みジョブを表示' }}
        </button>
      </div>
      <DataTable :columns="['ID', 'Type', 'Repository', 'State', 'Updated']">
        <tr v-for="job in data ?? []" :key="job.id">
          <td>
            <RouterLink class="table-link" :to="`/jobs/${job.id}`">{{ job.id }}</RouterLink>
            <p class="text-muted">{{ job.title }}</p>
          </td>
          <td>{{ job.type }}</td>
          <td>{{ job.repository }} #{{ job.githubNumber }}</td>
          <td><StateBadge :state="job.state" /></td>
          <td>{{ formatDateTime(job.updatedAt) }}</td>
        </tr>
        <tr v-if="(data ?? []).length === 0">
          <td colspan="5" class="text-muted">
            {{ showDeletedOnly ? '削除済みジョブはまだありません。' : 'ジョブはまだありません。' }}
          </td>
        </tr>
      </DataTable>
    </AsyncState>
  </AppShell>
</template>
