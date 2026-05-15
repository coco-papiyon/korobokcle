<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import PanelCard from '@/components/PanelCard.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchJobDetail, submitDesignApproval, submitDesignRerun, submitFinalApproval, submitImplementationRerun } from '@/lib/api'
import { formatDateTime } from '@/lib/format'

const route = useRoute()
const jobID = computed(() => String(route.params.id))
const { data, isLoading, error, reload } = useAsyncData(() => fetchJobDetail(jobID.value))
const approvalState = ref<'idle' | 'saving' | 'error'>('idle')
const finalApprovalState = ref<'idle' | 'saving' | 'error'>('idle')
const approvalError = ref<string | null>(null)
const finalApprovalError = ref<string | null>(null)
const designRerunComment = ref('')
const implementationRerunComment = ref('')
const designRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const implementationRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const designRerunError = ref<string | null>(null)
const implementationRerunError = ref<string | null>(null)

type JobEventLike = {
  id: number
  stateTo: string
}

const designRerunStates = new Set(['waiting_design_approval', 'design_rejected'])
const implementationRerunStates = new Set(['waiting_final_approval', 'final_rejected'])

const prCreateInfo = computed(() => {
  const raw = data.value?.prCreateArtifact?.content
  if (!raw) {
    return null
  }
  try {
    return JSON.parse(raw) as { url?: string; repository?: string; branchName?: string; title?: string }
  } catch {
    return null
  }
})

const canReviewDesign = computed(() => data.value?.job.state === 'waiting_design_approval')
const canReviewImplementation = computed(() => data.value?.job.state === 'waiting_final_approval')

function findLatestRerunTargetEventId(events: JobEventLike[] | undefined, state: string | undefined, allowedStates: Set<string>) {
  if (!events || !state || !allowedStates.has(state)) {
    return null
  }

  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index]
    if (event.stateTo === state) {
      return event.id
    }
  }

  return null
}

const designRerunTargetEventId = computed(() =>
  findLatestRerunTargetEventId(data.value?.events, data.value?.job.state, designRerunStates),
)
const implementationRerunTargetEventId = computed(() =>
  findLatestRerunTargetEventId(data.value?.events, data.value?.job.state, implementationRerunStates),
)

async function sendApproval(status: 'approved' | 'rejected') {
  approvalState.value = 'saving'
  approvalError.value = null
  try {
    data.value = await submitDesignApproval(jobID.value, status, '')
    approvalState.value = 'idle'
    await reload()
  } catch (err) {
    approvalState.value = 'error'
    approvalError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function sendFinalApproval(status: 'approved' | 'rejected') {
  finalApprovalState.value = 'saving'
  finalApprovalError.value = null
  try {
    data.value = await submitFinalApproval(jobID.value, status, '')
    finalApprovalState.value = 'idle'
    await reload()
  } catch (err) {
    finalApprovalState.value = 'error'
    finalApprovalError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function rerunDesign() {
  designRerunState.value = 'saving'
  designRerunError.value = null
  try {
    data.value = await submitDesignRerun(jobID.value, designRerunComment.value)
    designRerunComment.value = ''
    designRerunState.value = 'idle'
    await reload()
  } catch (err) {
    designRerunState.value = 'error'
    designRerunError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function rerunImplementation() {
  implementationRerunState.value = 'saving'
  implementationRerunError.value = null
  try {
    data.value = await submitImplementationRerun(jobID.value, implementationRerunComment.value)
    implementationRerunComment.value = ''
    implementationRerunState.value = 'idle'
    await reload()
  } catch (err) {
    implementationRerunState.value = 'error'
    implementationRerunError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}
</script>

<template>
  <AppShell
    title="Job Detail"
    description="ジョブ状態、関連ブランチ、イベント履歴を確認するページです。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <template v-if="data">
        <section class="hero-grid">
          <PanelCard :title="data.job.id" description="Job summary">
            <div class="stack-sm">
              <StateBadge :state="data.job.state" />
              <p class="text-muted">{{ data.job.repository }} #{{ data.job.githubNumber }}</p>
              <p>{{ data.job.title }}</p>
              <p class="text-muted">Branch: <code>{{ data.job.branchName }}</code></p>
              <p class="text-muted">Watch Rule: <code>{{ data.job.watchRuleId }}</code></p>
            </div>
          </PanelCard>
          <PanelCard title="Flow" description="設計承認、実装成果物確認、最終承認をここから行えます。">
            <div class="stack-sm">
              <p class="text-muted">Current state: <code>{{ data.job.state }}</code></p>
              <template v-if="canReviewDesign">
                <div class="button-row">
                  <button class="button button-secondary" type="button" :disabled="approvalState === 'saving'" @click="sendApproval('rejected')">
                    Reject Design
                  </button>
                  <button class="button button-primary" type="button" :disabled="approvalState === 'saving'" @click="sendApproval('approved')">
                    Approve Design
                  </button>
                </div>
              </template>
              <p v-if="approvalState === 'error'" class="notice notice-danger">{{ approvalError }}</p>
              <template v-if="canReviewImplementation">
                <div class="button-row">
                  <button class="button button-secondary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('rejected')">
                    Reject Final
                  </button>
                  <button class="button button-primary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('approved')">
                    Approve Final
                  </button>
                </div>
                <p v-if="finalApprovalState === 'error'" class="notice notice-danger">{{ finalApprovalError }}</p>
              </template>
            </div>
          </PanelCard>
        </section>

        <PanelCard
          v-if="data.designArtifact"
          title="Design Artifact"
          description="生成された設計成果物です。承認前に内容を確認します。"
        >
          <div class="stack-sm">
            <p class="text-muted">{{ data.designArtifact.path }}</p>
            <pre class="artifact-view">{{ data.designArtifact.content }}</pre>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.implementationArtifact"
          title="Implementation Artifact"
          description="実装フェーズの成果物サマリです。最終承認前に確認します。"
        >
          <div class="stack-sm">
            <p class="text-muted">{{ data.implementationArtifact.path }}</p>
            <pre class="artifact-view">{{ data.implementationArtifact.content }}</pre>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.testReport"
          title="Test Report"
          description="設定された test profile の実行結果です。"
        >
          <div class="stack-sm">
            <p class="text-muted">{{ data.testReport.path }}</p>
            <pre class="artifact-view">{{ data.testReport.content }}</pre>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.prCreateArtifact"
          title="Pull Request"
          description="作成された PR の記録です。"
        >
          <div class="stack-sm">
            <p class="text-muted">{{ data.prCreateArtifact.path }}</p>
            <template v-if="prCreateInfo">
              <p v-if="prCreateInfo.title"><strong>{{ prCreateInfo.title }}</strong></p>
              <p v-if="prCreateInfo.repository" class="text-muted">Repository: <code>{{ prCreateInfo.repository }}</code></p>
              <p v-if="prCreateInfo.branchName" class="text-muted">Branch: <code>{{ prCreateInfo.branchName }}</code></p>
              <p v-if="prCreateInfo.url">
                <a class="table-link" :href="prCreateInfo.url" target="_blank" rel="noreferrer">Open Pull Request</a>
              </p>
            </template>
            <pre class="artifact-view">{{ data.prCreateArtifact.content }}</pre>
          </div>
        </PanelCard>

        <DataTable :columns="['When', 'Event', 'State', 'Payload', 'Action']">
          <tr v-for="event in data.events" :key="event.id">
            <td>{{ formatDateTime(event.createdAt) }}</td>
            <td>{{ event.eventType }}</td>
            <td>{{ event.stateFrom || '-' }} → {{ event.stateTo || '-' }}</td>
            <td><code>{{ event.payload }}</code></td>
            <td>
              <div v-if="designRerunTargetEventId === event.id" class="stack-sm">
                <textarea
                  v-model="designRerunComment"
                  class="field__control field__control-textarea"
                  :disabled="designRerunState === 'saving'"
                  placeholder="rerun comment"
                />
                <div class="button-row">
                  <button
                    class="button button-secondary"
                    type="button"
                    :disabled="designRerunState === 'saving'"
                    @click="rerunDesign"
                  >
                    Rerun Design
                  </button>
                </div>
                <p v-if="designRerunState === 'error'" class="notice notice-danger">{{ designRerunError }}</p>
              </div>
              <div v-else-if="implementationRerunTargetEventId === event.id" class="stack-sm">
                <textarea
                  v-model="implementationRerunComment"
                  class="field__control field__control-textarea"
                  :disabled="implementationRerunState === 'saving'"
                  placeholder="implementation rerun comment"
                />
                <div class="button-row">
                  <button
                    class="button button-secondary"
                    type="button"
                    :disabled="implementationRerunState === 'saving'"
                    @click="rerunImplementation"
                  >
                    Rerun Implementation
                  </button>
                </div>
                <p v-if="implementationRerunState === 'error'" class="notice notice-danger">{{ implementationRerunError }}</p>
              </div>
              <span v-else>-</span>
            </td>
          </tr>
          <tr v-if="data.events.length === 0">
            <td colspan="5" class="text-muted">イベントはまだありません。</td>
          </tr>
        </DataTable>
      </template>
    </AsyncState>
  </AppShell>
</template>