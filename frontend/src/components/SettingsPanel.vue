<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { AIProvider, WatchSettings } from '../types'

const settingsLoading = ref(false)
const settingsSaving = ref(false)
const settingsError = ref('')

const settingsForm = ref({
  repository: '',
  aiProvider: 'codex' as AIProvider,
  pollIntervalSeconds: 120,
  jobConcurrency: 4,
  implementationLoopCount: 3,
  baseBranch: 'main',
  branchNamePattern: 'issue_#<issue番号>',
  aiAllowedCommandsText: '',
  codexModelSelection: 'default',
  githubCopilotModelSelection: 'default',
  verificationAiProviderSelection: '' as AIProvider | '',
  verificationAiModelSelection: 'default',
  issueAiProviderSelection: '' as AIProvider | '',
  issueAiModelSelection: 'default',
  issueLabelIncludesText: '',
  issueLabelExcludesText: '',
  issueTitleContainsText: '',
  issueAuthorsText: '',
  issueAssigneesText: '',
  issueEnabled: true,
  prAiProviderSelection: '' as AIProvider | '',
  prAiModelSelection: 'default',
  prLabelIncludesText: '',
  prLabelExcludesText: '',
  prTitleContainsText: '',
  prAuthorsText: '',
  prAssigneesText: '',
  prEnabled: true,
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

const verificationEffectiveProvider = computed(
  () => settingsForm.value.verificationAiProviderSelection || settingsForm.value.aiProvider,
)
const verificationActiveModelOptions = computed(() => modelOptions[verificationEffectiveProvider.value])
const verificationActiveModelSelection = computed({
  get() {
    const selection = settingsForm.value.verificationAiModelSelection
    return verificationActiveModelOptions.value.some((option) => option.value === selection) ? selection : 'default'
  },
  set(value: string) {
    const normalized = verificationActiveModelOptions.value.some((option) => option.value === value)
      ? value
      : 'default'
    settingsForm.value.verificationAiModelSelection = normalized
  },
})

const issueEffectiveProvider = computed(() => settingsForm.value.issueAiProviderSelection || settingsForm.value.aiProvider)
const issueActiveModelOptions = computed(() => modelOptions[issueEffectiveProvider.value])
const issueActiveModelSelection = computed({
  get() {
    const selection = settingsForm.value.issueAiModelSelection
    return issueActiveModelOptions.value.some((option) => option.value === selection) ? selection : 'default'
  },
  set(value: string) {
    const normalized = issueActiveModelOptions.value.some((option) => option.value === value) ? value : 'default'
    settingsForm.value.issueAiModelSelection = normalized
  },
})

const prEffectiveProvider = computed(() => settingsForm.value.prAiProviderSelection || settingsForm.value.aiProvider)
const prActiveModelOptions = computed(() => modelOptions[prEffectiveProvider.value])
const prActiveModelSelection = computed({
  get() {
    const selection = settingsForm.value.prAiModelSelection
    return prActiveModelOptions.value.some((option) => option.value === selection) ? selection : 'default'
  },
  set(value: string) {
    const normalized = prActiveModelOptions.value.some((option) => option.value === value) ? value : 'default'
    settingsForm.value.prAiModelSelection = normalized
  },
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

function splitLines(value: string) {
  return value
    .split(/\r?\n/)
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0)
}

function joinLines(values: string[]) {
  return values.join('\n')
}

function settingsToForm(settings: WatchSettings) {
  const codexModel = settings.models?.codex
  const githubCopilotModel = settings.models?.githubCopilot
  const issueModel = settings.issue?.aiModel
  const pullRequestModel = settings.pullRequest?.aiModel
  settingsForm.value.repository = settings.repository ?? ''
  settingsForm.value.aiProvider = settings.aiProvider ?? 'codex'
  settingsForm.value.pollIntervalSeconds = settings.pollIntervalSeconds ?? 120
  settingsForm.value.jobConcurrency = settings.jobConcurrency ?? 4
  settingsForm.value.implementationLoopCount = settings.implementationLoopCount ?? 3
  settingsForm.value.baseBranch = settings.baseBranch?.trim() || 'main'
  settingsForm.value.branchNamePattern = settings.branchNamePattern?.trim() || 'issue_#<issue番号>'
  settingsForm.value.aiAllowedCommandsText = joinLines(settings.aiAllowedCommands ?? settings.codexAllowedCommands ?? [])
  settingsForm.value.codexModelSelection = codexModel?.mode === 'custom' && codexModel.value ? codexModel.value : 'default'
  settingsForm.value.githubCopilotModelSelection =
    githubCopilotModel?.mode === 'custom' && githubCopilotModel.value ? githubCopilotModel.value : 'default'
  settingsForm.value.verificationAiProviderSelection = settings.verificationAiProvider ?? ''
  settingsForm.value.verificationAiModelSelection =
    settings.verificationAiModel?.mode === 'custom' && settings.verificationAiModel.value
      ? settings.verificationAiModel.value
      : 'default'
  settingsForm.value.issueAiProviderSelection = settings.issue?.aiProvider ?? ''
  settingsForm.value.issueAiModelSelection = issueModel?.mode === 'custom' && issueModel.value ? issueModel.value : 'default'
  settingsForm.value.issueLabelIncludesText = joinCSV(settings.issue?.labelIncludes ?? [])
  settingsForm.value.issueLabelExcludesText = joinCSV(settings.issue?.labelExcludes ?? [])
  settingsForm.value.issueTitleContainsText = joinCSV(settings.issue?.titleContains ?? [])
  settingsForm.value.issueAuthorsText = joinCSV(settings.issue?.authors ?? [])
  settingsForm.value.issueAssigneesText = joinCSV(settings.issue?.assignees ?? [])
  settingsForm.value.issueEnabled = settings.issue?.enabled ?? true
  settingsForm.value.prAiProviderSelection = settings.pullRequest?.aiProvider ?? ''
  settingsForm.value.prAiModelSelection =
    pullRequestModel?.mode === 'custom' && pullRequestModel.value ? pullRequestModel.value : 'default'
  settingsForm.value.prLabelIncludesText = joinCSV(settings.pullRequest?.labelIncludes ?? [])
  settingsForm.value.prLabelExcludesText = joinCSV(settings.pullRequest?.labelExcludes ?? [])
  settingsForm.value.prTitleContainsText = joinCSV(settings.pullRequest?.titleContains ?? [])
  settingsForm.value.prAuthorsText = joinCSV(settings.pullRequest?.authors ?? [])
  settingsForm.value.prAssigneesText = joinCSV(settings.pullRequest?.assignees ?? [])
  settingsForm.value.prEnabled = settings.pullRequest?.enabled ?? true
}

function formToSettings(): WatchSettings {
  const codexSelection = settingsForm.value.codexModelSelection
  const githubCopilotSelection = settingsForm.value.githubCopilotModelSelection
  const verificationSelection = settingsForm.value.verificationAiModelSelection
  return {
    repository: settingsForm.value.repository.trim(),
    aiProvider: settingsForm.value.aiProvider,
    pollIntervalSeconds:
      Number.isFinite(settingsForm.value.pollIntervalSeconds) && settingsForm.value.pollIntervalSeconds > 0
        ? Math.floor(settingsForm.value.pollIntervalSeconds)
        : 120,
    jobConcurrency:
      Number.isFinite(settingsForm.value.jobConcurrency) && settingsForm.value.jobConcurrency > 0
        ? Math.floor(settingsForm.value.jobConcurrency)
        : 4,
    implementationLoopCount:
      Number.isFinite(settingsForm.value.implementationLoopCount) && settingsForm.value.implementationLoopCount > 0
        ? Math.min(10, Math.floor(settingsForm.value.implementationLoopCount))
        : 3,
    baseBranch: settingsForm.value.baseBranch.trim() || 'main',
    branchNamePattern: settingsForm.value.branchNamePattern.trim() || 'issue_#<issue番号>',
    aiAllowedCommands: splitLines(settingsForm.value.aiAllowedCommandsText),
    models: {
      codex: codexSelection === 'default' ? { mode: 'default', value: '' } : { mode: 'custom', value: codexSelection },
      githubCopilot:
        githubCopilotSelection === 'default'
          ? { mode: 'default', value: '' }
          : { mode: 'custom', value: githubCopilotSelection },
    },
    verificationAiProvider: settingsForm.value.verificationAiProviderSelection || undefined,
    verificationAiModel:
      verificationSelection === 'default'
        ? { mode: 'default', value: '' }
        : { mode: 'custom', value: verificationSelection },
    issue: {
      enabled: settingsForm.value.issueEnabled,
      aiProvider: settingsForm.value.issueAiProviderSelection || undefined,
      aiModel:
        settingsForm.value.issueAiModelSelection === 'default'
          ? { mode: 'default', value: '' }
          : { mode: 'custom', value: settingsForm.value.issueAiModelSelection },
      labelIncludes: splitCSV(settingsForm.value.issueLabelIncludesText),
      labelExcludes: splitCSV(settingsForm.value.issueLabelExcludesText),
      titleContains: splitCSV(settingsForm.value.issueTitleContainsText),
      authors: splitCSV(settingsForm.value.issueAuthorsText),
      assignees: splitCSV(settingsForm.value.issueAssigneesText),
    },
    pullRequest: {
      enabled: settingsForm.value.prEnabled,
      aiProvider: settingsForm.value.prAiProviderSelection || undefined,
      aiModel:
        settingsForm.value.prAiModelSelection === 'default'
          ? { mode: 'default', value: '' }
          : { mode: 'custom', value: settingsForm.value.prAiModelSelection },
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
  } catch (err) {
    settingsError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    settingsSaving.value = false
  }
}

onMounted(() => {
  void loadSettings()
})

defineExpose({
  saveSettings,
  settingsSaving,
})
</script>

<template>
  <p v-if="settingsError" class="error">{{ settingsError }}</p>

  <div class="panel__title-row">
    <h2>プロバイダー設定</h2>
    <span class="panel__hint">GET / PUT /api/settings</span>
  </div>

  <div class="form settings-grid">
    <div class="model-grid settings-section--full">
      <div class="model-card">
        <h4>実装者</h4>
        <label class="field">
          <span>AI プロバイダー</span>
          <select v-model="settingsForm.aiProvider" class="control">
            <option v-for="(label, value) in aiProviderLabels" :key="value" :value="value">
              {{ label }} ({{ value }})
            </option>
          </select>
        </label>
        <label class="field">
          <span>モデル</span>
          <select v-model="activeModelSelection" class="control">
            <option v-for="option in activeModelOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
      </div>

      <div class="model-card">
        <h4>検証者</h4>
        <label class="field">
          <span>AI プロバイダー</span>
          <select v-model="settingsForm.verificationAiProviderSelection" class="control">
            <option value="">実装者設定を使用</option>
            <option v-for="(label, value) in aiProviderLabels" :key="value" :value="value">
              {{ label }} ({{ value }})
            </option>
          </select>
        </label>
        <label class="field">
          <span>モデル</span>
          <select v-model="verificationActiveModelSelection" class="control">
            <option value="default">実装者設定を使用</option>
            <option v-for="option in verificationActiveModelOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
      </div>
    </div>

    <label class="field field--full">
      <span>AI 許可コマンド</span>
      <textarea
        v-model="settingsForm.aiAllowedCommandsText"
        class="control"
        rows="4"
        placeholder="npm ci&#10;npm test&#10;go test ./..."
      ></textarea>
      <span class="field-note">Codex / Copilot CLI の承認要求を自動承認するコマンド。1行に1コマンドで指定する。</span>
    </label>
  </div>

  <div class="panel__title-row">
    <h2>実行設定</h2>
    <span class="panel__hint">Job runtime defaults</span>
  </div>

  <div class="form settings-grid">
    <label class="field field--full">
      <span>ジョブ多重度</span>
      <input v-model.number="settingsForm.jobConcurrency" class="control" type="number" min="1" step="1" />
      <span class="field-note">同時に実行できるジョブ数。デフォルトは4。</span>
    </label>

    <label class="field field--full">
      <span>実装・検証ループ回数</span>
      <input v-model.number="settingsForm.implementationLoopCount" class="control" type="number" min="1" max="10" step="1" />
      <span class="field-note">実装者と検証者が反復する最大回数。検証に合格した時点で終了する。デフォルトは3。</span>
    </label>

    <label class="field field--full">
      <span>ベースブランチ</span>
      <input v-model="settingsForm.baseBranch" class="control" type="text" placeholder="main" />
      <span class="field-note">PR 作成時に `gh pr create --base` へ渡すブランチ名。</span>
    </label>

    <label class="field field--full">
      <span>ブランチ名ルール</span>
      <input v-model="settingsForm.branchNamePattern" class="control" type="text" placeholder="issue_#&lt;issue番号&gt;" />
      <span class="field-note">&lt;issue番号&gt; を issue 番号に置き換えてブランチを作成する。</span>
    </label>
  </div>

  <div class="panel__title-row">
    <h2>監視設定</h2>
    <span class="panel__hint">Issue / PR watch</span>
  </div>
  <p class="field-note">対象リポジトリと Issue / PR の絞り込み条件を設定する。</p>

  <div class="form settings-grid">
    <label class="field field--full">
      <span>監視リポジトリ</span>
      <input v-model="settingsForm.repository" class="control" type="text" placeholder="owner/repository" />
    </label>

    <label class="field field--full">
      <span>監視間隔（秒）</span>
      <input v-model.number="settingsForm.pollIntervalSeconds" class="control" type="number" min="1" step="1" />
    </label>

    <div class="settings-section" :class="{ 'settings-section--disabled': !settingsForm.issueEnabled }">
      <div class="settings-section__header">
        <h3>Issue 条件</h3>
        <label class="settings-section__toggle">
          <input v-model="settingsForm.issueEnabled" type="checkbox" />
          <span>有効にする</span>
        </label>
      </div>
      <label class="field">
        <span>含めるラベル</span>
        <input v-model="settingsForm.issueLabelIncludesText" class="control" type="text" placeholder="bug, ai:design" :disabled="!settingsForm.issueEnabled" />
      </label>
      <label class="field">
        <span>除外するラベル</span>
        <input v-model="settingsForm.issueLabelExcludesText" class="control" type="text" placeholder="wip, draft" :disabled="!settingsForm.issueEnabled" />
      </label>
      <label class="field">
        <span>タイトルに含める語</span>
        <input v-model="settingsForm.issueTitleContainsText" class="control" type="text" placeholder="fix, refactor" :disabled="!settingsForm.issueEnabled" />
      </label>
      <label class="field">
        <span>作者</span>
        <input v-model="settingsForm.issueAuthorsText" class="control" type="text" placeholder="alice, bob" :disabled="!settingsForm.issueEnabled" />
      </label>
      <label class="field">
        <span>担当者</span>
        <input v-model="settingsForm.issueAssigneesText" class="control" type="text" placeholder="carol, dave" :disabled="!settingsForm.issueEnabled" />
      </label>
      <label class="field">
        <span>AI プロバイダー</span>
        <select v-model="settingsForm.issueAiProviderSelection" class="control" :disabled="!settingsForm.issueEnabled">
          <option value="">プロバイダー設定を使用</option>
          <option v-for="(label, value) in aiProviderLabels" :key="value" :value="value">
            {{ label }} ({{ value }})
          </option>
        </select>
      </label>
      <label class="field">
        <span>モデル選択</span>
        <select v-model="issueActiveModelSelection" class="control" :disabled="!settingsForm.issueEnabled">
          <option value="default">プロバイダー設定を使用</option>
          <option v-for="option in issueActiveModelOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
        <p class="field-note">
          <template v-if="settingsForm.issueAiProviderSelection === ''">
            Issue 監視で使う AI 設定は、上の「プロバイダー設定」を引き継ぐ。
          </template>
          <template v-else>
            ここで選んだプロバイダーのモデルを使う。`default` は、そのプロバイダーの「プロバイダー設定」を使う。
          </template>
        </p>
      </label>
    </div>

    <div class="settings-section" :class="{ 'settings-section--disabled': !settingsForm.prEnabled }">
      <div class="settings-section__header">
        <h3>PR 条件</h3>
        <label class="settings-section__toggle">
          <input v-model="settingsForm.prEnabled" type="checkbox" />
          <span>有効にする</span>
        </label>
      </div>
      <label class="field">
        <span>含めるラベル</span>
        <input v-model="settingsForm.prLabelIncludesText" class="control" type="text" placeholder="ready, review" :disabled="!settingsForm.prEnabled" />
      </label>
      <label class="field">
        <span>除外するラベル</span>
        <input v-model="settingsForm.prLabelExcludesText" class="control" type="text" placeholder="wip, draft" :disabled="!settingsForm.prEnabled" />
      </label>
      <label class="field">
        <span>タイトルに含める語</span>
        <input v-model="settingsForm.prTitleContainsText" class="control" type="text" placeholder="fix, update" :disabled="!settingsForm.prEnabled" />
      </label>
      <label class="field">
        <span>作者</span>
        <input v-model="settingsForm.prAuthorsText" class="control" type="text" placeholder="alice, bob" :disabled="!settingsForm.prEnabled" />
      </label>
      <label class="field">
        <span>担当者</span>
        <input v-model="settingsForm.prAssigneesText" class="control" type="text" placeholder="carol, dave" :disabled="!settingsForm.prEnabled" />
      </label>
      <label class="field">
        <span>AI プロバイダー</span>
        <select v-model="settingsForm.prAiProviderSelection" class="control" :disabled="!settingsForm.prEnabled">
          <option value="">プロバイダー設定を使用</option>
          <option v-for="(label, value) in aiProviderLabels" :key="value" :value="value">
            {{ label }} ({{ value }})
          </option>
        </select>
      </label>
      <label class="field">
        <span>モデル選択</span>
        <select v-model="prActiveModelSelection" class="control" :disabled="!settingsForm.prEnabled">
          <option value="default">プロバイダー設定を使用</option>
          <option v-for="option in prActiveModelOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
        <p class="field-note">
          <template v-if="settingsForm.prAiProviderSelection === ''">
            PR 監視で使う AI 設定は、上の「プロバイダー設定」を引き継ぐ。
          </template>
          <template v-else>
            ここで選んだプロバイダーのモデルを使う。`default` は、そのプロバイダーの「プロバイダー設定」を使う。
          </template>
        </p>
      </label>
    </div>
  </div>
</template>
