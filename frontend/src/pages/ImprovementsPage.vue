<script setup lang="ts">
import { computed, ref } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { approveImprovement, fetchImprovementDetail, fetchImprovements, saveImprovementDraft } from '@/lib/api'
import { formatDateTime, formatStateLabel } from '@/lib/format'
import { UNKNOWN_ERROR_MESSAGE } from '@/lib/ui-text'
import type { ImprovementDetail, ImprovementItem } from '@/types'

const { data, isLoading, error, reload } = useAsyncData(fetchImprovements)
const selectedItem = ref<ImprovementItem | null>(null)
const detail = ref<ImprovementDetail | null>(null)
const detailState = ref<'idle' | 'loading' | 'error'>('idle')
const detailError = ref<string | null>(null)
const draftText = ref('')
const approveComment = ref('')
const actionState = ref<'idle' | 'saving' | 'error'>('idle')
const actionError = ref<string | null>(null)

const canApprove = computed(() => detail.value?.state !== 'no_improvement_needed' && draftText.value.trim().length > 0)
const canMarkNoImprovement = computed(() => detail.value?.state !== 'no_improvement_needed')

async function openDetail(item: ImprovementItem) {
  selectedItem.value = item
  detailState.value = 'loading'
  detailError.value = null
  actionState.value = 'idle'
  actionError.value = null
  approveComment.value = ''
  try {
    const loaded = await fetchImprovementDetail(item.repository, item.issueNumber)
    detail.value = loaded
    draftText.value = loaded.draft
    detailState.value = 'idle'
  } catch (err) {
    detail.value = null
    detailState.value = 'error'
    detailError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

function closeDetail() {
  selectedItem.value = null
  detail.value = null
  draftText.value = ''
  approveComment.value = ''
  detailState.value = 'idle'
  detailError.value = null
  actionState.value = 'idle'
  actionError.value = null
}

async function persistDraft() {
  if (!detail.value) {
    return
  }
  actionState.value = 'saving'
  actionError.value = null
  try {
    const saved = await saveImprovementDraft(detail.value.repository, detail.value.issueNumber, draftText.value)
    detail.value = saved
    draftText.value = saved.draft
    actionState.value = 'idle'
    await reload()
  } catch (err) {
    actionState.value = 'error'
    actionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function submitApproval() {
  if (!detail.value || !canApprove.value) {
    return
  }
  actionState.value = 'saving'
  actionError.value = null
  try {
    const approved = await approveImprovement(detail.value.repository, detail.value.issueNumber, 'approved', approveComment.value)
    detail.value = approved
    draftText.value = approved.draft
    actionState.value = 'idle'
    await reload()
  } catch (err) {
    actionState.value = 'error'
    actionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function markNoImprovementNeeded() {
  if (!detail.value || !canMarkNoImprovement.value) {
    return
  }
  actionState.value = 'saving'
  actionError.value = null
  try {
    const updated = await approveImprovement(detail.value.repository, detail.value.issueNumber, 'no_improvement_needed', approveComment.value)
    detail.value = updated
    draftText.value = updated.draft
    actionState.value = 'idle'
    await reload()
  } catch (err) {
    actionState.value = 'error'
    actionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}
</script>

<template>
  <AppShell title="改善点" description="改善案の一覧、詳細、編集、承認を管理します。">
    <AsyncState :is-loading="isLoading" :error="error">
      <DataTable :columns="['リポジトリ', 'Issue', '関連 Job', 'タイトル', '状態', '理由', '更新日時']">
        <tr
          v-for="item in data ?? []"
          :key="`${item.repository}-${item.issueNumber}`"
          class="data-table__row-clickable"
          @click="openDetail(item)"
        >
          <td>{{ item.repository }}</td>
          <td>#{{ item.issueNumber }}</td>
          <td>{{ item.relatedJobId || '-' }}</td>
          <td>{{ item.title }}</td>
          <td><StateBadge :state="item.state">{{ formatStateLabel(item.state) }}</StateBadge></td>
          <td>{{ item.decisionReason || '-' }}</td>
          <td>{{ formatDateTime(item.updatedAt) }}</td>
        </tr>
      </DataTable>
      <p v-if="(data ?? []).length === 0" class="text-muted">改善点はありません。</p>

      <div v-if="selectedItem" class="modal-backdrop" @click.self="closeDetail">
        <div class="modal-panel">
          <div class="modal-panel__header">
            <div>
              <h3 class="modal-panel__title">改善点詳細</h3>
              <p class="text-muted">{{ selectedItem.repository }} / #{{ selectedItem.issueNumber }}</p>
            </div>
            <button class="button button-secondary" type="button" @click="closeDetail">閉じる</button>
          </div>

          <p v-if="detailState === 'loading'" class="text-muted">詳細を読み込んでいます...</p>
          <p v-else-if="detailState === 'error'" class="notice notice-danger">{{ detailError }}</p>
          <template v-else-if="detail">
            <div class="stack-md">
              <div class="panel">
                <p><strong>タイトル:</strong> {{ detail.title }}</p>
                <p><strong>状態:</strong> {{ formatStateLabel(detail.state) }}</p>
                <p><strong>関連 Job:</strong> {{ detail.relatedJobId || '-' }}</p>
                <p><strong>適用 phase:</strong> {{ detail.phases.length > 0 ? detail.phases.join(', ') : '-' }}</p>
                <p class="text-muted">改善ブランチ: <code>{{ detail.improvementBranch }}</code></p>
                <p class="text-muted">承認前作業ディレクトリ: <code>{{ detail.improvementWorkDir }}</code></p>
                <p class="text-muted">draft 保存先: <code>{{ detail.draftPath }}</code></p>
              </div>

              <label class="field">
                <span class="field__label">元コメント</span>
                <textarea class="field__control field__control--textarea" :value="detail.input" rows="8" readonly />
              </label>

              <label v-if="detail.state !== 'no_improvement_needed'" class="field">
                <span class="field__label">改善案 draft</span>
                <textarea v-model="draftText" class="field__control field__control--textarea" rows="14" />
              </label>

              <div v-else class="notice notice-success">
                恒久改善不要: {{ detail.decisionReason || 'AI が継続的な改善は不要と判断しました。' }}
              </div>

              <label v-if="detail.state !== 'no_improvement_needed'" class="field">
                <span class="field__label">承認コメント</span>
                <textarea v-model="approveComment" class="field__control field__control--textarea" rows="4" />
              </label>

              <div v-if="actionState === 'error'" class="notice notice-danger">{{ actionError }}</div>

              <div class="modal-actions">
                <button v-if="detail.state !== 'no_improvement_needed'" class="button button-secondary" type="button" :disabled="actionState === 'saving'" @click="persistDraft">
                  {{ actionState === 'saving' ? '保存中...' : 'draft を保存' }}
                </button>
                <div class="modal-actions__right">
                  <button
                    v-if="canMarkNoImprovement"
                    class="button button-secondary"
                    type="button"
                    :disabled="actionState === 'saving'"
                    @click="markNoImprovementNeeded"
                  >
                    {{ actionState === 'saving' ? '更新中...' : '改善不要' }}
                  </button>
                  <button
                    v-if="detail.state !== 'no_improvement_needed'"
                    class="button button-primary"
                    type="button"
                    :disabled="actionState === 'saving' || !canApprove"
                    @click="submitApproval"
                  >
                    {{ actionState === 'saving' ? '承認中...' : '承認' }}
                  </button>
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </AsyncState>
  </AppShell>
</template>
