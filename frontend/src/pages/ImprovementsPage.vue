<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import PanelCard from '@/components/PanelCard.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import {
  approveImprovement,
  fetchAppConfig,
  fetchImprovementDetail,
  fetchImprovements,
  regenerateImprovement,
  pushImprovement,
  rejectImprovement,
  saveImprovementDraft,
  saveImprovementWorkspace,
} from '@/lib/api'
import { formatDateTime } from '@/lib/format'
import { UNKNOWN_ERROR_MESSAGE } from '@/lib/ui-text'
import type { Artifact, ImprovementDetail, ImprovementSummary } from '@/types'
import { computed, ref } from 'vue'

const { data: appConfig } = useAsyncData(fetchAppConfig)
const refreshIntervalMs = computed(() => {
  const seconds = appConfig.value?.screenRefreshInterval ?? 0
  return seconds > 0 ? seconds * 1000 : 0
})

const { data, isLoading, error, reload } = useAsyncData(fetchImprovements, {
  pollIntervalMs: refreshIntervalMs,
})
const items = computed(() => data.value ?? [])

const selectedJobId = ref('')
const selectedDetail = ref<ImprovementDetail | null>(null)
const detailState = ref<'idle' | 'loading' | 'error'>('idle')
const detailError = ref<string | null>(null)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const approvalState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const regenerateState = ref<'idle' | 'saving' | 'error'>('idle')
const workspaceState = ref<'idle' | 'saving' | 'pushing' | 'error'>('idle')
const actionError = ref<string | null>(null)
const draftBody = ref('')
const notesBody = ref('')
const approvalComment = ref('')
const workspaceFiles = ref<Artifact[]>([])

function resetDetailActionState() {
  saveState.value = 'idle'
  approvalState.value = 'idle'
  regenerateState.value = 'idle'
  workspaceState.value = 'idle'
  actionError.value = null
}

function improvementStatusLabel(status: string) {
  switch (status) {
    case 'generating':
      return 'AIによる改善案作成中'
    case 'draft_created':
      return '下書き確認待ち'
    case 'no_improvement_needed':
      return '改善不要'
    case 'approved':
      return '承認済み'
    case 'rejected':
      return '却下済み'
    default:
      return status || '-'
  }
}

function improvementDecisionLabel(decision: string) {
  switch (decision) {
    case 'draft_created':
      return '下書き作成済み'
    case 'no_improvement_needed':
      return '改善不要'
    case 'approved':
      return '承認済み'
    case 'rejected':
      return '却下済み'
    default:
      return decision || '-'
  }
}

function improvementStatusClass(status: string) {
  switch (status) {
    case 'approved':
      return 'state-badge state-badge--success'
    case 'rejected':
      return 'state-badge state-badge--danger'
    case 'generating':
    case 'draft_created':
      return 'state-badge state-badge--warning'
    case 'no_improvement_needed':
      return 'state-badge state-badge--neutral'
    default:
      return 'state-badge state-badge--neutral'
  }
}

function formatPhases(phases?: string[]) {
  if (!phases || phases.length === 0) {
    return '-'
  }
  return phases.join(', ')
}

function previewText(item: ImprovementSummary) {
  return item.reason?.trim() || item.title || '-'
}

function formatUpdatedAt(value?: string) {
  if (!value) {
    return '-'
  }
  return formatDateTime(value)
}

const detailImprovementStatus = computed(() => {
  if (regenerateState.value === 'saving') {
    return 'AIによる改善案作成中'
  }
  if (saveState.value === 'saving') {
    return '改善案を保存中'
  }
  if (approvalState.value === 'saving') {
    return '改善案を更新中'
  }
  return improvementStatusLabel(selectedDetail.value?.summary.status ?? '')
})

const detailImprovementStatusClass = computed(() => {
  if (regenerateState.value === 'saving' || saveState.value === 'saving' || approvalState.value === 'saving') {
    return 'state-badge state-badge--warning'
  }
  return improvementStatusClass(selectedDetail.value?.summary.status ?? '')
})

function setWorkspaceFiles(files?: Artifact[]) {
  workspaceFiles.value = (files ?? []).map((file) => ({
    path: file.path,
    content: file.content,
  }))
}

async function openDetail(jobId: string) {
  selectedJobId.value = jobId
  selectedDetail.value = null
  detailState.value = 'loading'
  detailError.value = null
  resetDetailActionState()
  approvalComment.value = ''
  try {
    const detail = await fetchImprovementDetail(jobId)
    selectedDetail.value = detail
    draftBody.value = detail.draft?.content ?? detail.result?.content ?? ''
    notesBody.value = detail.notes?.content ?? ''
    setWorkspaceFiles(detail.workspace)
    detailState.value = 'idle'
  } catch (err) {
    detailState.value = 'error'
    detailError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

function closeDetail() {
  selectedJobId.value = ''
  selectedDetail.value = null
  detailState.value = 'idle'
  detailError.value = null
  draftBody.value = ''
  notesBody.value = ''
  approvalComment.value = ''
  workspaceFiles.value = []
  resetDetailActionState()
}

async function saveDraft() {
  if (!selectedJobId.value) {
    return
  }
  saveState.value = 'saving'
  actionError.value = null
  try {
    selectedDetail.value = await saveImprovementDraft(selectedJobId.value, draftBody.value, notesBody.value)
    saveState.value = 'saved'
    closeDetail()
    await reload()
  } catch (err) {
    saveState.value = 'error'
    actionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function submitApproval(status: 'approved' | 'rejected') {
  if (!selectedJobId.value) {
    return
  }
  approvalState.value = 'saving'
  actionError.value = null
  try {
    selectedDetail.value = status === 'approved'
      ? await approveImprovement(selectedJobId.value, approvalComment.value, draftBody.value)
      : await rejectImprovement(selectedJobId.value, approvalComment.value, draftBody.value)
    approvalState.value = 'saved'
    if (status === 'approved') {
      setWorkspaceFiles(selectedDetail.value.workspace)
    } else {
      closeDetail()
    }
    await reload()
  } catch (err) {
    approvalState.value = 'error'
    actionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function rerunGeneration() {
  if (!selectedJobId.value) {
    return
  }
  regenerateState.value = 'saving'
  actionError.value = null
  try {
    selectedDetail.value = await regenerateImprovement(selectedJobId.value, selectedDetail.value?.summary.sourceEventType ?? '')
    draftBody.value = selectedDetail.value.draft?.content ?? ''
    notesBody.value = selectedDetail.value.notes?.content ?? ''
    regenerateState.value = 'idle'
    closeDetail()
    await reload()
  } catch (err) {
    regenerateState.value = 'error'
    actionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

function hasEditableWorkspaceFiles() {
  return workspaceFiles.value.length > 0
}

async function pushWorkspace() {
  if (!selectedJobId.value || workspaceFiles.value.length === 0) {
    return
  }
  workspaceState.value = 'saving'
  actionError.value = null
  try {
    const saved = await saveImprovementWorkspace(
      selectedJobId.value,
      workspaceFiles.value.map((file) => ({ path: file.path, content: file.content })),
    )
    selectedDetail.value = saved
    setWorkspaceFiles(saved.workspace)
    workspaceState.value = 'pushing'
    const pushed = await pushImprovement(selectedJobId.value)
    selectedDetail.value = pushed
    setWorkspaceFiles(pushed.workspace)
    workspaceState.value = 'idle'
    closeDetail()
    await reload()
  } catch (err) {
    workspaceState.value = 'error'
    actionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}
</script>

<template>
  <AppShell
    title="改善一覧"
    description="repository 単位の改善案と改善不要判定を一覧で確認し、そのまま編集と承認を行えます。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <PanelCard title="改善一覧" description="`generating` / `draft_created` / `approved` / `rejected` / `no_improvement_needed` をまとめて表示します。">
        <DataTable :columns="['Repository', 'Issue', 'ステータス', '判定', 'Phase', '更新日時', '概要']">
          <tr v-for="item in items" :key="item.jobId">
            <td>
              <button class="artifact-link" type="button" @click="openDetail(item.jobId)">{{ item.repository }}</button>
            </td>
            <td>
              <RouterLink class="table-link" :to="`/jobs/${item.jobId}`">#{{ item.issueNumber }}</RouterLink>
            </td>
            <td><span :class="improvementStatusClass(item.status)">{{ improvementStatusLabel(item.status) }}</span></td>
            <td>{{ improvementDecisionLabel(item.decision) }}</td>
            <td>{{ formatPhases(item.phases) }}</td>
            <td>{{ formatUpdatedAt(item.updatedAt) }}</td>
            <td>{{ previewText(item) }}</td>
          </tr>
        </DataTable>
      </PanelCard>

      <div v-if="selectedJobId" class="modal-backdrop" @click.self="closeDetail">
        <div class="modal-panel improvement-modal-panel">
          <div class="modal-panel__header">
            <div>
              <h3 class="modal-panel__title">改善詳細</h3>
              <p v-if="selectedDetail" class="text-muted">
                {{ selectedDetail.summary.repository }} / #{{ selectedDetail.summary.issueNumber }} / {{ improvementDecisionLabel(selectedDetail.summary.decision) }}
              </p>
            </div>
            <button class="button button-secondary" type="button" @click="closeDetail">閉じる</button>
          </div>

          <p v-if="detailState === 'loading'" class="text-muted">改善詳細を取得しています...</p>
          <p v-else-if="detailState === 'error'" class="notice notice-danger">{{ detailError }}</p>

          <div v-else-if="selectedDetail" class="stack-md">
            <section class="panel improvement-overview-panel">
              <div class="improvement-overview__header">
                <h2>概要</h2>
                <span :class="detailImprovementStatusClass">{{ detailImprovementStatus }}</span>
              </div>
              <div class="stack-sm">
                <p><strong>Source:</strong> {{ selectedDetail.summary.sourceEventType || '-' }}</p>
                <p><strong>Phase:</strong> {{ formatPhases(selectedDetail.summary.phases) }}</p>
                <p><strong>Reason:</strong> {{ selectedDetail.summary.reason || '-' }}</p>
              </div>
            </section>

            <PanelCard v-if="selectedDetail.summary.decision === 'no_improvement_needed'" title="改善不要">
              <pre class="artifact-view">{{ selectedDetail.summary.reason || '理由なし' }}</pre>
            </PanelCard>

            <PanelCard v-if="selectedDetail.input" title="入力">
              <pre class="artifact-view">{{ selectedDetail.input.content }}</pre>
            </PanelCard>

            <PanelCard title="改善案">
              <textarea v-model="draftBody" class="field__control field__control--textarea improvement-editor" />
            </PanelCard>

            <div class="field">
              <label class="field__label" for="improvement-approval-comment">承認コメント</label>
              <textarea id="improvement-approval-comment" v-model="approvalComment" class="field__control field__control--textarea" />
            </div>

            <PanelCard
              v-if="selectedDetail.summary.decision === 'approved'"
              title="承認後の修正"
              description="`.improvement/*.md` を確認してから push します。AI の修正内容をそのまま push せず、必要ならこの画面で編集します。"
            >
              <div v-if="hasEditableWorkspaceFiles()" class="stack-md">
                <section v-for="file in workspaceFiles" :key="file.path" class="stack-sm">
                  <div class="improvement-overview__header">
                    <h3>{{ file.path }}</h3>
                  </div>
                  <textarea
                    v-model="file.content"
                    class="field__control field__control--textarea improvement-editor"
                    spellcheck="false"
                  />
                </section>
                <div class="modal-actions">
                  <button
                    class="button button-primary"
                    type="button"
                    :disabled="workspaceState === 'saving' || workspaceState === 'pushing'"
                    @click="pushWorkspace"
                  >
                    {{ workspaceState === 'saving' ? '保存中...' : workspaceState === 'pushing' ? 'push中...' : 'push' }}
                  </button>
                </div>
              </div>
              <p v-else class="text-muted">編集可能な `.md` ファイルはまだありません。</p>
            </PanelCard>

            <p v-if="saveState === 'saved'" class="notice notice-success">改善案を保存しました。</p>
            <p v-if="approvalState === 'saved'" class="notice notice-success">改善案を更新しました。</p>
            <p v-if="actionError" class="notice notice-danger">{{ actionError }}</p>

            <div class="modal-actions">
              <button class="button button-secondary" type="button" :disabled="saveState === 'saving'" @click="saveDraft">
                {{ saveState === 'saving' ? '保存中...' : '保存' }}
              </button>
              <button class="button button-secondary" type="button" :disabled="regenerateState === 'saving'" @click="rerunGeneration">
                {{ regenerateState === 'saving' ? '再生成中...' : '再生成' }}
              </button>
              <div class="modal-actions__right">
                <button class="button button-danger" type="button" :disabled="approvalState === 'saving'" @click="submitApproval('rejected')">
                  却下
                </button>
                <button class="button button-primary" type="button" :disabled="approvalState === 'saving'" @click="submitApproval('approved')">
                  承認
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </AsyncState>
  </AppShell>
</template>
