<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

type JobKind = 'issue_design' | 'issue_implementation' | 'pr_review' | 'pr_feedback'

type Job = {
  id: string
  kind: JobKind
  state: string
  repository: string
  number: number
  title: string
}

type SearchCondition = {
  labelIncludes: string[]
  labelExcludes: string[]
  titleContains: string[]
  authors: string[]
  assignees: string[]
}

type AIProvider = 'codex' | 'github_copilot'

type ModelSelection = {
  mode: 'default' | 'custom'
  value: string
}

type AIModels = {
  codex: ModelSelection
  githubCopilot: ModelSelection
}

type WatchSettings = {
  repository: string
  aiProvider: AIProvider
  pollIntervalSeconds: number
  models: AIModels
  issue: SearchCondition
  pullRequest: SearchCondition
}

const jobs = ref<Job[]>([])
const selectedJobId = ref('')
const activeTab = ref<'settings' | 'jobs' | 'detail'>('jobs')
const loadingJobs = ref(false)
const settingsLoading = ref(false)
const settingsSaving = ref(false)
const error = ref('')
const settingsError = ref('')
const detailLoading = ref(false)
const detailError = ref('')
const detailJob = ref<Job | null>(null)
let refreshTimer: number | undefined

const settingsForm = ref({
  repository: '',
  aiProvider: 'codex',
  pollIntervalSeconds: 120,
  codexModelSelection: 'default',
  githubCopilotModelSelection: 'default',
  issueLabelIncludesText: '',
  issueLabelExcludesText: '',
  issueTitleContainsText: '',
  issueAuthorsText: '',
  issueAssigneesText: '',
  prLabelIncludesText: '',
  prLabelExcludesText: '',
  prTitleContainsText: '',
  prAuthorsText: '',
  prAssigneesText: '',
})

const aiProviderLabels: Record<AIProvider, string> = {
  codex: 'Codex',
  github_copilot: 'GitHub Copilot',
}

const modelOptions: Record<AIProvider, Array<{ value: string; label: string }>> = {
  codex: [
    { value: 'default', label: 'デフォルト' },
    { value: 'gpt-5.5', label: 'gpt-5.5' },
    { value: 'gpt-5.4', label: 'gpt-5.4' },
    { value: 'gpt-5.4-mini', label: 'gpt-5.4-mini' },
  ],
  github_copilot: [
    { value: 'default', label: 'デフォルト' },
    { value: 'claude-sonnet-4.6', label: 'claude-sonnet-4.6' },
    { value: 'claude-opus-4.6', label: 'claude-opus-4.6' },
    { value: 'gpt-5.4', label: 'gpt-5.4' },
    { value: 'gpt-5-mini', label: 'gpt-5-mini' },
    { value: 'gpt-4.1', label: 'gpt-4.1' },
  ],
}

const stateLabels: Record<string, string> = {
  detected: '検知済み',
  design_running: '設計中',
  design_ready: '設計完了',
  design_approved: '設計承認済み',
  implementation_running: '実装中',
  implementation_ready: '実装完了',
  implementation_approved: '実装承認済み',
  pr_created: 'PR済み',
  pr_review_comment: 'PRレビューコメント状態',
  review_fix_design_running: 'レビュー指摘検討中',
  review_fix_design_ready: 'レビュー指摘検討済み',
  review_fix_design_approved: 'レビュー検討承認済み',
  review_fix_implementation_running: 'レビュー指摘修正中',
  review_fix_implementation_ready: 'レビュー指摘修正完了',
  review_fix_implementation_approved: 'レビュー指摘修正承認済み',
  review_fixed: 'レビュー指摘修正済み',
  review_running: 'レビュー中',
  review_ready: 'レビュー完了',
  review_approved: 'レビュー承認済み',
  failed: '失敗',
}

const activeModelOptions = computed(() => modelOptions[settingsForm.value.aiProvider])

const activeModelSelection = computed({
  get() {
    const selection =
      settingsForm.value.aiProvider === 'codex'
        ? settingsForm.value.codexModelSelection
        : settingsForm.value.githubCopilotModelSelection
    return activeModelOptions.value.some((option) => option.value === selection) ? selection : 'default'
  },
  set(value: string) {
    const normalized = activeModelOptions.value.some((option) => option.value === value) ? value : 'default'
    if (settingsForm.value.aiProvider === 'codex') {
      settingsForm.value.codexModelSelection = normalized
    } else {
      settingsForm.value.githubCopilotModelSelection = normalized
    }
  },
})

const sortedJobs = computed(() => {
  return [...jobs.value].sort((a, b) => {
    if (a.repository !== b.repository) return a.repository.localeCompare(b.repository)
    if (a.number !== b.number) return a.number - b.number
    return a.kind.localeCompare(b.kind)
  })
})

function splitCSV(value: string) {
  return value
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0)
}

function joinCSV(values: string[]) {
  return values.join(', ')
}

function settingsToForm(settings: WatchSettings) {
  const codexModel = settings.models?.codex
  const githubCopilotModel = settings.models?.githubCopilot
  settingsForm.value.repository = settings.repository ?? ''
  settingsForm.value.aiProvider = settings.aiProvider ?? 'codex'
  settingsForm.value.pollIntervalSeconds = settings.pollIntervalSeconds ?? 120
  settingsForm.value.codexModelSelection = codexModel?.mode === 'custom' && codexModel.value ? codexModel.value : 'default'
  settingsForm.value.githubCopilotModelSelection =
    githubCopilotModel?.mode === 'custom' && githubCopilotModel.value ? githubCopilotModel.value : 'default'
  settingsForm.value.issueLabelIncludesText = joinCSV(settings.issue?.labelIncludes ?? [])
  settingsForm.value.issueLabelExcludesText = joinCSV(settings.issue?.labelExcludes ?? [])
  settingsForm.value.issueTitleContainsText = joinCSV(settings.issue?.titleContains ?? [])
  settingsForm.value.issueAuthorsText = joinCSV(settings.issue?.authors ?? [])
  settingsForm.value.issueAssigneesText = joinCSV(settings.issue?.assignees ?? [])
  settingsForm.value.prLabelIncludesText = joinCSV(settings.pullRequest?.labelIncludes ?? [])
  settingsForm.value.prLabelExcludesText = joinCSV(settings.pullRequest?.labelExcludes ?? [])
  settingsForm.value.prTitleContainsText = joinCSV(settings.pullRequest?.titleContains ?? [])
  settingsForm.value.prAuthorsText = joinCSV(settings.pullRequest?.authors ?? [])
  settingsForm.value.prAssigneesText = joinCSV(settings.pullRequest?.assignees ?? [])
}

function formToSettings(): WatchSettings {
  const codexSelection = settingsForm.value.codexModelSelection
  const githubCopilotSelection = settingsForm.value.githubCopilotModelSelection
  return {
    repository: settingsForm.value.repository.trim(),
    aiProvider: settingsForm.value.aiProvider,
    pollIntervalSeconds: Number.isFinite(settingsForm.value.pollIntervalSeconds) && settingsForm.value.pollIntervalSeconds > 0
      ? Math.floor(settingsForm.value.pollIntervalSeconds)
      : 120,
    models: {
      codex: codexSelection === 'default' ? { mode: 'default', value: '' } : { mode: 'custom', value: codexSelection },
      githubCopilot:
        githubCopilotSelection === 'default'
          ? { mode: 'default', value: '' }
          : { mode: 'custom', value: githubCopilotSelection },
    },
    issue: {
      labelIncludes: splitCSV(settingsForm.value.issueLabelIncludesText),
      labelExcludes: splitCSV(settingsForm.value.issueLabelExcludesText),
      titleContains: splitCSV(settingsForm.value.issueTitleContainsText),
      authors: splitCSV(settingsForm.value.issueAuthorsText),
      assignees: splitCSV(settingsForm.value.issueAssigneesText),
    },
    pullRequest: {
      labelIncludes: splitCSV(settingsForm.value.prLabelIncludesText),
      labelExcludes: splitCSV(settingsForm.value.prLabelExcludesText),
      titleContains: splitCSV(settingsForm.value.prTitleContainsText),
      authors: splitCSV(settingsForm.value.prAuthorsText),
      assignees: splitCSV(settingsForm.value.prAssigneesText),
    },
  }
}

async function loadSettings() {
  settingsLoading.value = true
  settingsError.value = ''
  try {
    const res = await fetch('/api/settings')
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const payload = (await res.json()) as WatchSettings
    settingsToForm(payload)
  } catch (err) {
    settingsError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    settingsLoading.value = false
  }
}

async function saveSettings() {
  settingsSaving.value = true
  settingsError.value = ''
  try {
    const res = await fetch('/api/settings', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(formToSettings()),
    })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    const payload = (await res.json()) as WatchSettings
    settingsToForm(payload)
    await loadJobs()
  } catch (err) {
    settingsError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    settingsSaving.value = false
  }
}

async function loadJobs() {
  loadingJobs.value = true
  error.value = ''
  try {
    const res = await fetch('/api/jobs')
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const payload = (await res.json()) as { jobs?: Job[] }
    jobs.value = payload.jobs ?? []
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    loadingJobs.value = false
  }
}

async function loadJobDetail(id: string) {
  detailLoading.value = true
  detailError.value = ''
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(id)}`)
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    detailJob.value = (await res.json()) as Job
  } catch (err) {
    detailError.value = err instanceof Error ? err.message : 'unknown error'
    detailJob.value = null
  } finally {
    detailLoading.value = false
  }
}

function jobStateLabel(state: string) {
  return stateLabels[state] ?? state
}

function selectJob(job: Job) {
  selectedJobId.value = job.id
  activeTab.value = 'detail'
  void loadJobDetail(job.id)
}

function selectTab(tab: 'settings' | 'jobs' | 'detail') {
  activeTab.value = tab
  if (tab === 'detail' && selectedJobId.value && (!detailJob.value || detailJob.value.id !== selectedJobId.value)) {
    void loadJobDetail(selectedJobId.value)
  }
}

onMounted(() => {
  void loadSettings()
  void loadJobs()
  refreshTimer = window.setInterval(() => {
    void loadJobs()
  }, 5000)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer)
  }
})
</script>

<template>
  <div class="app-shell">
    <main class="dashboard">
      <section class="hero hero--compact">
        <p class="eyebrow">korobokcle</p>
        <div class="hero__header">
          <div>
            <p class="lede">
              監視するリポジトリと、Issue / PR ごとの検索条件をここで設定する。
            </p>
          </div>
          <div class="hero__actions">
            <button class="button button--ghost" type="button" @click="loadSettings" :disabled="settingsLoading">
              {{ settingsLoading ? '読込中' : '再読込' }}
            </button>
            <button class="button" type="button" @click="saveSettings" :disabled="settingsSaving">
              {{ settingsSaving ? '保存中' : '保存' }}
            </button>
          </div>
        </div>

        <p v-if="settingsError" class="error">{{ settingsError }}</p>
      </section>

      <section class="panel">
        <div class="tabs" role="tablist" aria-label="korobokcle views">
          <button
            class="tab"
            :class="{ 'tab--active': activeTab === 'settings' }"
            type="button"
            role="tab"
            :aria-selected="activeTab === 'settings'"
            @click="selectTab('settings')"
          >
            設定
          </button>
          <button
            class="tab"
            :class="{ 'tab--active': activeTab === 'jobs' }"
            type="button"
            role="tab"
            :aria-selected="activeTab === 'jobs'"
            @click="selectTab('jobs')"
          >
            ジョブ一覧
          </button>
          <button
            class="tab"
            :class="{ 'tab--active': activeTab === 'detail' }"
            type="button"
            role="tab"
            :aria-selected="activeTab === 'detail'"
            @click="selectTab('detail')"
          >
            詳細
          </button>
        </div>

        <div v-show="activeTab === 'settings'" class="tab-panel" role="tabpanel">
          <div class="panel__title-row">
            <h2>監視設定</h2>
            <span class="panel__hint">GET / PUT /api/settings</span>
          </div>

          <div class="form settings-grid">
            <label class="field field--full">
              <span>監視リポジトリ</span>
              <input v-model="settingsForm.repository" class="control" type="text" placeholder="owner/repository" />
            </label>

            <label class="field field--full">
              <span>AI プロバイダー</span>
              <select v-model="settingsForm.aiProvider" class="control">
                <option v-for="(label, value) in aiProviderLabels" :key="value" :value="value">
                  {{ label }} ({{ value }})
                </option>
              </select>
            </label>

            <label class="field field--full">
              <span>監視間隔（秒）</span>
              <input v-model.number="settingsForm.pollIntervalSeconds" class="control" type="number" min="1" step="1" />
            </label>

            <div class="settings-section settings-section--full">
              <h3>モデル</h3>
              <label class="field">
                <span>モデル選択</span>
                <select v-model="activeModelSelection" class="control">
                  <option v-for="option in activeModelOptions" :key="option.value" :value="option.value">
                    {{ option.label }}
                  </option>
                </select>
              </label>
              <p class="field-note">
                プロバイダーに応じて候補が切り替わる。<template v-if="settingsForm.aiProvider === 'codex'">Codex</template><template v-else>GitHub Copilot</template>
                の既定値もここに含まれる。
              </p>
            </div>

            <div class="settings-section">
              <h3>Issue 条件</h3>
              <label class="field">
                <span>含めるラベル</span>
                <input v-model="settingsForm.issueLabelIncludesText" class="control" type="text" placeholder="bug, ai:design" />
              </label>
              <label class="field">
                <span>除外するラベル</span>
                <input v-model="settingsForm.issueLabelExcludesText" class="control" type="text" placeholder="wip, draft" />
              </label>
              <label class="field">
                <span>タイトルに含める語</span>
                <input v-model="settingsForm.issueTitleContainsText" class="control" type="text" placeholder="fix, refactor" />
              </label>
              <label class="field">
                <span>作者</span>
                <input v-model="settingsForm.issueAuthorsText" class="control" type="text" placeholder="alice, bob" />
              </label>
              <label class="field">
                <span>担当者</span>
                <input v-model="settingsForm.issueAssigneesText" class="control" type="text" placeholder="carol, dave" />
              </label>
            </div>

            <div class="settings-section">
              <h3>PR 条件</h3>
              <label class="field">
                <span>含めるラベル</span>
                <input v-model="settingsForm.prLabelIncludesText" class="control" type="text" placeholder="ready, review" />
              </label>
              <label class="field">
                <span>除外するラベル</span>
                <input v-model="settingsForm.prLabelExcludesText" class="control" type="text" placeholder="wip, draft" />
              </label>
              <label class="field">
                <span>タイトルに含める語</span>
                <input v-model="settingsForm.prTitleContainsText" class="control" type="text" placeholder="fix, update" />
              </label>
              <label class="field">
                <span>作者</span>
                <input v-model="settingsForm.prAuthorsText" class="control" type="text" placeholder="alice, bob" />
              </label>
              <label class="field">
                <span>担当者</span>
                <input v-model="settingsForm.prAssigneesText" class="control" type="text" placeholder="carol, dave" />
              </label>
            </div>
          </div>
        </div>

        <div v-show="activeTab === 'jobs'" class="tab-panel" role="tabpanel">
          <div class="panel__title-row">
            <h2>現在のジョブ</h2>
            <span class="panel__hint">{{ sortedJobs.length }} 件</span>
          </div>

          <p v-if="error" class="error">{{ error }}</p>

          <div v-if="sortedJobs.length === 0" class="empty-state">
            まだジョブがありません。
          </div>

          <div v-else class="job-list">
            <article
              v-for="job in sortedJobs"
              :key="job.id"
              class="job-card job-card--selectable"
              :class="{ 'job-card--active': selectedJobId === job.id }"
              @click="selectJob(job)"
            >
              <div class="job-card__top">
                <div>
                  <p class="job-card__repo">{{ job.repository }}</p>
                  <h3>{{ job.title || `#${job.number}` }}</h3>
                </div>
                <span class="chip">{{ jobStateLabel(job.state) }}</span>
              </div>

              <dl class="meta">
                <div>
                  <dt>Kind</dt>
                  <dd>{{ job.kind }}</dd>
                </div>
                <div>
                  <dt>ID</dt>
                  <dd>{{ job.id }}</dd>
                </div>
                <div>
                  <dt>Number</dt>
                  <dd>#{{ job.number }}</dd>
                </div>
              </dl>
            </article>
          </div>
        </div>

        <div v-show="activeTab === 'detail'" class="tab-panel" role="tabpanel">
          <div class="panel__title-row">
            <h2>ジョブ詳細</h2>
            <span class="panel__hint">GET /api/jobs/:id</span>
          </div>

          <div v-if="detailLoading" class="empty-state">読み込み中...</div>
          <div v-else-if="detailError" class="error">{{ detailError }}</div>
          <div v-else-if="detailJob" class="detail">
            <div class="detail__header">
              <div>
                <p class="job-card__repo">{{ detailJob.repository }}</p>
                <h3>{{ detailJob.title || `#${detailJob.number}` }}</h3>
              </div>
              <span class="chip">{{ jobStateLabel(detailJob.state) }}</span>
            </div>

            <dl class="detail__meta">
              <div>
                <dt>ID</dt>
                <dd>{{ detailJob.id }}</dd>
              </div>
              <div>
                <dt>Kind</dt>
                <dd>{{ detailJob.kind }}</dd>
              </div>
              <div>
                <dt>State</dt>
                <dd>{{ detailJob.state }}</dd>
              </div>
              <div>
                <dt>Repository</dt>
                <dd>{{ detailJob.repository }}</dd>
              </div>
              <div>
                <dt>Number</dt>
                <dd>#{{ detailJob.number }}</dd>
              </div>
            </dl>

          </div>

          <div v-else class="empty-state">一覧からジョブを選択してください。</div>
        </div>
      </section>
    </main>
  </div>
</template>
