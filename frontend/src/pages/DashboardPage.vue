<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import PanelCard from '@/components/PanelCard.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, fetchJobs } from '@/lib/api'
import { formatDateTime } from '@/lib/format'

const { data, isLoading, error } = useAsyncData(fetchJobs)
const { data: appConfig } = useAsyncData(fetchAppConfig)
</script>

<template>
  <AppShell
    title="korobokcle"
    description="GitHub Issue / Pull Request automation orchestration with approval-driven job control."
  >
    <section class="hero-grid">
      <PanelCard title="Jobs" description="承認待ち、進行中、失敗ジョブを一箇所で追跡します。" />
      <PanelCard title="AI Provider" description="現在の実行 provider と設定変更の導線です。">
        <div class="status-inline">
          <StateBadge :state="`provider:${appConfig?.provider ?? 'mock'}`" />
          <RouterLink class="button button-secondary" to="/settings">Open Settings</RouterLink>
        </div>
        <p class="text-muted">
          現在値: <strong>{{ appConfig?.provider ?? 'mock' }}</strong>
        </p>
      </PanelCard>
    </section>

    <AsyncState :is-loading="isLoading" :error="error">
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
