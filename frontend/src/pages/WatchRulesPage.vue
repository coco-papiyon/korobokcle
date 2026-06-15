<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, fetchSkillSets, fetchTestProfiles, fetchToolCommands, fetchWatchRules, saveWatchRules } from '@/lib/api'
import { modelOptionsForProvider, watchRuleProviderOptions } from '@/lib/provider-options'
import { UNKNOWN_ERROR_MESSAGE } from '@/lib/ui-text'
import type { AppConfig, ProjectFieldFilter, WatchRule, WatchRuleForm } from '@/types'

const { data, isLoading, error, reload } = useAsyncData(fetchWatchRules)
const { data: appConfig } = useAsyncData(fetchAppConfig)
const { data: skillSets } = useAsyncData(fetchSkillSets)
const { data: testProfiles } = useAsyncData(fetchTestProfiles)
const { data: toolCommands } = useAsyncData(fetchToolCommands)
const forms = ref<WatchRuleForm[]>([])
const selectedRuleId = ref('')
const nextRuleLocalID = ref(1)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)

watch(
  data,
  (rules) => {
    forms.value = (rules ?? []).map(toForm)
    if (!selectedRuleId.value || !forms.value.some((rule) => rule.localID === selectedRuleId.value)) {
      selectedRuleId.value = forms.value[0]?.localID ?? ''
    }
  },
  { immediate: true },
)

const selectedRule = computed(() => forms.value.find((rule) => rule.localID === selectedRuleId.value) ?? null)
const availableRepositoryEntries = computed(() => appConfig.value?.monitoredRepositories ?? [])
const availableRepositories = computed(() =>
  availableRepositoryEntries.value.map((entry) => entry.repository.trim()).filter(Boolean),
)
const availableModelOptions = computed(() => {
  const config = appConfig.value as AppConfig | null | undefined
  const providerCatalog = config?.providers ?? []
  const selectedProvider = selectedRule.value?.provider?.trim() || config?.provider || ''
  return modelOptionsForProvider(providerCatalog, selectedProvider, selectedRule.value?.model ?? '')
})
const selectedRuleInvalidRepositories = computed(() => {
  const rule = selectedRule.value
  if (!rule) {
    return []
  }
  const allowed = new Set(availableRepositories.value)
  if (!rule.selectedRepository) {
    return []
  }
  return allowed.has(rule.selectedRepository) ? [] : [rule.selectedRepository]
})

function toForm(rule: WatchRule): WatchRuleForm {
  const normalizedProjectFilters = normalizeProjectFilters(rule.projectFilters)
  const selectedRepository = (rule.repositories ?? []).map((value) => value.trim()).find(Boolean) ?? ''
  return {
    localID: `watch-rule-${nextRuleLocalID.value++}`,
    ...rule,
    target: normalizeTarget(rule.target),
    repositories: selectedRepository ? [selectedRepository] : [],
    selectedRepository,
    projectName: rule.projectName ?? '',
    projectFilters: normalizedProjectFilters,
    labels: rule.labels ?? [],
    authors: rule.authors ?? [],
    assignees: rule.assignees ?? [],
    reviewers: rule.reviewers ?? [],
    repositoriesText: selectedRepository,
    projectFiltersText: formatProjectFilters(normalizedProjectFilters),
    labelsText: (rule.labels ?? []).join(', '),
    authorsText: (rule.authors ?? []).join(', '),
    assigneesText: (rule.assignees ?? []).join(', '),
    reviewersText: (rule.reviewers ?? []).join(', '),
  }
}

function fromForm(rule: WatchRuleForm): WatchRule {
  const repository = rule.selectedRepository.trim()
  return {
    id: rule.id.trim(),
    name: rule.name.trim(),
    repositories: repository ? [repository] : [],
    target: normalizeTarget(rule.target),
    projectName: rule.projectName.trim(),
    labels: splitCSV(rule.labelsText),
    projectFilters: parseProjectFilters(rule.projectFiltersText),
    titlePattern: rule.titlePattern.trim(),
    authors: splitCSV(rule.authorsText),
    assignees: splitCSV(rule.assigneesText),
    reviewers: splitCSV(rule.reviewersText),
    excludeDraftPR: rule.excludeDraftPR,
    provider: rule.provider.trim(),
    model: rule.model.trim(),
    skillSet: rule.skillSet.trim(),
    testProfile: rule.testProfile.trim(),
    toolCommand: rule.toolCommand.trim(),
    enabled: rule.enabled,
  }
}

function normalizeTarget(value: string): string {
  const target = value.trim()
  if (target === 'pull_request_review_comment') {
    return 'pull_request_review'
  }
  if (!target) {
    return 'issue'
  }
  return target
}

function parseProjectFilters(value: string): ProjectFieldFilter[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const separatorIndex = line.indexOf(':')
      if (separatorIndex < 0) {
        return { field: line, values: [] }
      }
      return {
        field: line.slice(0, separatorIndex).trim(),
        values: splitCSV(line.slice(separatorIndex + 1)),
      }
    })
    .filter((filter) => filter.field)
}

function normalizeProjectFilters(value: unknown): ProjectFieldFilter[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value
    .map((item) => {
      if (!item || typeof item !== 'object') {
        return null
      }
      const record = item as Record<string, unknown>
      const fieldValue = record.field ?? record.Field
      const valuesValue = record.values ?? record.Values
      const field = typeof fieldValue === 'string' ? fieldValue.trim() : ''
      const values = Array.isArray(valuesValue)
        ? valuesValue.map((entry) => String(entry).trim()).filter(Boolean)
        : []
      if (!field) {
        return null
      }
      return { field, values }
    })
    .filter((filter): filter is ProjectFieldFilter => filter !== null)
}

function formatProjectFilters(filters: ProjectFieldFilter[]): string {
  return filters
    .map((filter) => {
      const field = (filter.field ?? '').trim()
      if (!field) {
        return ''
      }
      const values = (filter.values ?? []).map((value) => value.trim()).filter(Boolean)
      if (!values.length) {
        return field
      }
      return `${field}: ${values.join(', ')}`
    })
    .filter(Boolean)
    .join('\n')
}

function splitCSV(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function selectRule(ruleID: string) {
  selectedRuleId.value = ruleID
}

function addRule() {
  const defaultRepositories = availableRepositories.value.slice(0, 1)
  const suffix = nextRuleLocalID.value
  const rule: WatchRuleForm = {
    localID: `watch-rule-${nextRuleLocalID.value++}`,
    id: `rule-${suffix}`,
    name: `新規ルール ${suffix}`,
    repositories: [...defaultRepositories],
    selectedRepository: defaultRepositories[0] ?? '',
    repositoriesText: defaultRepositories[0] ?? '',
    target: 'issue',
    projectName: '',
    projectFilters: [],
    projectFiltersText: '',
    labels: [],
    labelsText: '',
    titlePattern: '',
    authors: [],
    authorsText: '',
    assignees: [],
    assigneesText: '',
    reviewers: [],
    reviewersText: '',
    excludeDraftPR: true,
    provider: '',
    model: '',
    skillSet: 'default',
    testProfile: 'go-default',
    toolCommand: '',
    enabled: false,
  }
  forms.value = [...forms.value, rule]
  selectedRuleId.value = rule.localID
  saveState.value = 'idle'
  saveError.value = null
}

function updateSelectedRepository(repository: string) {
  if (!selectedRule.value) {
    return
  }
  selectedRule.value.selectedRepository = repository
  selectedRule.value.repositories = repository ? [repository] : []
  selectedRule.value.repositoriesText = repository
}

function removeSelectedRule() {
  if (!selectedRule.value) {
    return
  }
  forms.value = forms.value.filter((rule) => rule.localID !== selectedRule.value?.localID)
  selectedRuleId.value = forms.value[0]?.localID ?? ''
  saveState.value = 'idle'
  saveError.value = null
}

async function persistRules() {
  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveWatchRules(forms.value.map(fromForm))
    forms.value = saved.map(toForm)
    selectedRuleId.value = forms.value[0]?.localID ?? ''
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
    title="監視ルール"
    description="GitHubの監視対象および検出した際のAI/動作を設定します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="watch-layout">
        <aside class="panel rule-list">
          <div class="rule-list__header">
            <div>
              <h2>ルール一覧</h2>
              <p class="text-muted">監視対象のセットを選択します。</p>
            </div>
            <button class="button button-primary" type="button" @click="addRule">ルールを追加</button>
          </div>

          <div class="stack-sm">
            <button
              v-for="rule in forms"
              :key="rule.localID"
              class="rule-item"
              :class="{ 'rule-item--active': selectedRuleId === rule.localID }"
              type="button"
              @click="selectRule(rule.localID)"
            >
              <div class="rule-item__head">
                <strong>{{ rule.name }}</strong>
                <StateBadge :state="rule.enabled ? 'enabled' : 'disabled'" />
              </div>
              <p class="text-muted">{{ rule.repositoriesText || 'リポジトリ未設定' }}</p>
              <p class="text-muted">プロバイダー: {{ rule.provider || '設定を使用' }} / モデル: {{ rule.model || '設定を使用' }}</p>
            </button>
          </div>
        </aside>

        <section class="panel rule-editor">
          <div class="rule-editor__header">
            <div>
              <h2>ルール編集</h2>
              <p class="text-muted">監視条件を編集し、設定ファイルへ保存します。</p>
            </div>
            <div class="button-row">
              <button class="button button-secondary" type="button" :disabled="!selectedRule" @click="removeSelectedRule">
                削除
              </button>
              <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistRules">
                {{ saveState === 'saving' ? '保存中...' : 'ルールを保存' }}
              </button>
            </div>
          </div>

          <template v-if="selectedRule">
            <div class="form-grid">
              <label class="field">
                <span class="field__label">名前</span>
                <input v-model="selectedRule.name" class="field__control" type="text" />
              </label>

              <label class="field">
                <span class="field__label">対象</span>
                <select v-model="selectedRule.target" class="field__control">
                  <option value="issue">Issue</option>
                  <option value="issue_project">Issue（Project）</option>
                  <option value="pull_request">プルリクエスト</option>
                  <option value="pull_request_review">PR レビュー</option>
                </select>
              </label>

              <label class="field field-checkbox">
                <input v-model="selectedRule.enabled" type="checkbox" />
                <span>有効</span>
              </label>

              <label class="field field-full">
                <span class="field__label">リポジトリ</span>
                <select
                  v-if="availableRepositoryEntries.length"
                  :value="selectedRule.selectedRepository"
                  class="field__control"
                  @change="updateSelectedRepository(($event.target as HTMLSelectElement).value)"
                >
                  <option value="">リポジトリを選択</option>
                  <option v-for="entry in availableRepositoryEntries" :key="entry.repository" :value="entry.repository">
                    {{ entry.repository }} (実装 worker 数: {{ entry.implementationWorkers }})
                  </option>
                </select>
                <p v-else class="text-muted">設定画面で監視対象リポジトリを追加してください。</p>
                <p v-if="selectedRuleInvalidRepositories.length" class="notice notice-danger">
                  未登録のリポジトリが含まれています: {{ selectedRuleInvalidRepositories.join(', ') }}
                </p>
              </label>

              <label v-if="selectedRule.target === 'issue_project'" class="field field-full">
                <span class="field__label">プロジェクト名</span>
                <input
                  v-model="selectedRule.projectName"
                  class="field__control"
                  type="text"
                  placeholder="Roadmap"
                />
              </label>

              <label v-if="selectedRule.target === 'issue_project'" class="field field-full">
                <span class="field__label">プロジェクトフィールドフィルタ</span>
                <textarea
                  v-model="selectedRule.projectFiltersText"
                  class="field__control field__control--textarea"
                  rows="4"
                  placeholder="Status: Todo, In Progress&#10;Iteration: Sprint 12"
                />
                <p class="text-muted">1 行につき `Field: value1, value2`。プロジェクト名が空なら任意の project を対象にします。</p>
              </label>

              <label class="field field-full">
                <span class="field__label">ラベル</span>
                <input v-model="selectedRule.labelsText" class="field__control" type="text" placeholder="ai:design, backend" />
              </label>

              <label class="field field-full">
                <span class="field__label">タイトルパターン</span>
                <input v-model="selectedRule.titlePattern" class="field__control" type="text" placeholder="^feat:" />
              </label>

              <label class="field">
                <span class="field__label">作成者</span>
                <input v-model="selectedRule.authorsText" class="field__control" type="text" placeholder="alice, bob" />
              </label>

              <label class="field">
                <span class="field__label">担当者</span>
                <input v-model="selectedRule.assigneesText" class="field__control" type="text" placeholder="carol, dave" />
              </label>

              <label class="field">
                <span class="field__label">レビュー担当</span>
                <input v-model="selectedRule.reviewersText" class="field__control" type="text" placeholder="erin, frank" />
              </label>

              <div class="field field-full" style="display:grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-4);">
                <label class="field">
                  <span class="field__label">スキルセット</span>
                  <select v-model="selectedRule.skillSet" class="field__control">
                    <option v-for="skillSet in skillSets ?? []" :key="skillSet.name" :value="skillSet.name">
                      {{ skillSet.name }}
                    </option>
                  </select>
                </label>

                <label class="field">
                  <span class="field__label">テストプロファイル</span>
                  <select v-model="selectedRule.testProfile" class="field__control">
                    <option value="">なし</option>
                    <option v-for="profile in testProfiles ?? []" :key="profile.name" :value="profile.name">
                      {{ profile.name }}
                    </option>
                  </select>
                </label>
              </div>

              <div class="field field-full" style="display:grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-4);">
                <label class="field">
                  <span class="field__label">プロバイダー</span>
                  <select v-model="selectedRule.provider" class="field__control">
                    <option v-for="option in watchRuleProviderOptions(appConfig?.providers ?? [])" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>
                </label>

                <label class="field">
                  <span class="field__label">モデル</span>
                  <select v-model="selectedRule.model" class="field__control">
                    <option v-for="option in availableModelOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>
                </label>
              </div>

              <label class="field field-full">
                <span class="field__label">ツールコマンド</span>
                <select v-model="selectedRule.toolCommand" class="field__control">
                  <option value="">なし</option>
                  <option v-for="command in toolCommands ?? []" :key="command.name" :value="command.name">
                    {{ command.name }} ({{ command.resident ? '常駐' : '単発' }})
                  </option>
                </select>
              </label>

              <label class="field field-checkbox field-full">
                <input v-model="selectedRule.excludeDraftPR" type="checkbox" />
                <span>Draft PR を除外</span>
              </label>
            </div>

            <div v-if="saveState === 'saved'" class="notice notice-success">watch-rules.yaml を更新しました。</div>
            <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
          </template>

          <div v-else class="notice">ルールを追加するか、左側の一覧から選択してください。</div>
        </section>
      </section>
    </AsyncState>
  </AppShell>
</template>
