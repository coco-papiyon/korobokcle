<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import PanelCard from '@/components/PanelCard.vue'
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

const { data, isLoading, isRefreshing, error } = useAsyncData(fetchJobs, {
  pollIntervalMs: 5000,
  mergeData: mergeJobs,
})
const { data: appConfig, isRefreshing: isRefreshingAppConfig } = useAsyncData(fetchAppConfig, {
  pollIntervalMs: 5000,
})
</script>

<template>
  <AppShell
    title="korobokcle"
    description="GitHub Issue / Pull Request automation orchestration with approval-driven job control."
  >
    <section class="hero-grid">
      <PanelCard title="Jobs" description="承認待ち、進行中、失敗ジョブを一箇所で追跡します。" />
      <PanelCard title="AI Provider" description="現在の実行 provider を表示します。">
        <div class="status-inline">
          <StateBadge :state="`provider:${appConfig?.provider ?? 'mock'}`" />
          <StateBadge :state="`model:${appConfig?.model ?? 'default'}`" />
        </div>
        <p v-if="isRefreshingAppConfig" class="text-muted">Syncing settings...</p>
      </PanelCard>
    </section>

    <AsyncState :is-loading="isLoading" :error="error">
      <p v-if="isRefreshing" class="text-muted">Syncing jobs...</p>
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
          <td colspan="5" class="text-muted">ジョブはまだありません。</td>
        </tr>
      </DataTable>
    </AsyncState>
  </AppShell>
</template>
