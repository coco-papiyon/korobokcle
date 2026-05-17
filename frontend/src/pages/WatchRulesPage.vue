<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import PanelCard from '@/components/PanelCard.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { fetchAppConfig, fetchSkillSets, fetchWatchRules, saveWatchRules } from '@/lib/api'
import { modelOptionsForProvider, watchRuleProviderOptions } from '@/lib/provider-options'
import type { AppConfig, WatchRule, WatchRuleForm } from '@/types'

const { data, isLoading, error, reload } = useAsyncData(fetchWatchRules)
const { data: appConfig } = useAsyncData(fetchAppConfig)
const { data: skillSets } = useAsyncData(fetchSkillSets)
const forms = ref<WatchRuleForm[]>([])
const selectedRuleId = ref('')
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)

watch(
  data,
  (rules) => {
    forms.value = (rules ?? []).map(toForm)
    if (!selectedRuleId.value || !forms.value.some((rule) => rule.id === selectedRuleId.value)) {
      selectedRuleId.value = forms.value[0]?.id ?? ''
    }
  },
  { immediate: true },
)

const selectedRule = computed(() => forms.value.find((rule) => rule.id === selectedRuleId.value) ?? null)
const availableModelOptions = computed(() => {
  const config = appConfig.value as AppConfig | null | undefined
  const providerCatalog = config?.providers ?? []
  const selectedProvider = selectedRule.value?.provider?.trim() || config?.provider || ''
  return modelOptionsForProvider(providerCatalog, selectedProvider, selectedRule.value?.model ?? '', 'Use setting')
})

function toForm(rule: WatchRule): WatchRuleForm {
  return {
    ...rule,
    repositories: rule.repositories ?? [],
    labels: rule.labels ?? [],
    authors: rule.authors ?? [],
    assignees: rule.assignees ?? [],
    repositoriesText: (rule.repositories ?? []).join(', '),
    labelsText: (rule.labels ?? []).join(', '),
    authorsText: (rule.authors ?? []).join(', '),
    assigneesText: (rule.assignees ?? []).join(', '),
  }
}

function fromForm(rule: WatchRuleForm): WatchRule {
  return {
    id: rule.id.trim(),
    name: rule.name.trim(),
    repositories: splitCSV(rule.repositoriesText),
    target: rule.target,
    labels: splitCSV(rule.labelsText),
    titlePattern: rule.titlePattern.trim(),
    authors: splitCSV(rule.authorsText),
    assignees: splitCSV(rule.assigneesText),
    excludeDraftPR: rule.excludeDraftPR,
    provider: rule.provider.trim(),
    model: rule.model.trim(),
    skillSet: rule.skillSet.trim(),
    testProfile: rule.testProfile.trim(),
    enabled: rule.enabled,
  }
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
  const suffix = forms.value.length + 1
  const rule: WatchRuleForm = {
    id: `rule-${suffix}`,
    name: `New Rule ${suffix}`,
    repositories: [],
    repositoriesText: '',
    target: 'issue',
    labels: [],
    labelsText: '',
    titlePattern: '',
    authors: [],
    authorsText: '',
    assignees: [],
    assigneesText: '',
    excludeDraftPR: true,
    provider: '',
    model: '',
    skillSet: 'default',
    testProfile: 'go-default',
    enabled: false,
  }
  forms.value = [...forms.value, rule]
  selectedRuleId.value = rule.id
  saveState.value = 'idle'
  saveError.value = null
}

function removeSelectedRule() {
  if (!selectedRule.value) {
    return
  }
  forms.value = forms.value.filter((rule) => rule.id !== selectedRule.value?.id)
  selectedRuleId.value = forms.value[0]?.id ?? ''
  saveState.value = 'idle'
  saveError.value = null
}

async function persistRules() {
  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveWatchRules(forms.value.map(fromForm))
    forms.value = saved.map(toForm)
    selectedRuleId.value = forms.value[0]?.id ?? ''
    saveState.value = 'saved'
    await reload()
  } catch (err) {
    saveState.value = 'error'
    saveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}
</script>

<template>
  <AppShell
    title="Watch Rules"
    description="GitHub の監視対象、マッチ条件、実行スキルを Web 画面から設定します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="hero-grid">
        <PanelCard title="Rules" description="Issue / PR 監視対象を追加し、ラベルや対象リポジトリを調整します。" />
        <PanelCard title="Save Behavior" description="保存すると watch-rules.yaml に反映され、次回ポーリングから新しい設定が使われます。" />
      </section>

      <section class="watch-layout">
        <aside class="panel rule-list">
          <div class="rule-list__header">
            <div>
              <h2>Rule List</h2>
              <p class="text-muted">監視対象のセットを選択します。</p>
            </div>
            <button class="button button-primary" type="button" @click="addRule">Add Rule</button>
          </div>

          <div class="stack-sm">
            <button
              v-for="rule in forms"
              :key="rule.id"
              class="rule-item"
              :class="{ 'rule-item--active': selectedRuleId === rule.id }"
              type="button"
              @click="selectRule(rule.id)"
            >
              <div class="rule-item__head">
                <strong>{{ rule.name }}</strong>
                <StateBadge :state="rule.enabled ? 'enabled' : 'disabled'" />
              </div>
              <p class="text-muted">{{ rule.id }}</p>
              <p class="text-muted">{{ rule.repositoriesText || 'repository not set' }}</p>
              <p class="text-muted">Provider: {{ rule.provider || 'use setting' }} / Model: {{ rule.model || 'use setting' }}</p>
            </button>
          </div>
        </aside>

        <section class="panel rule-editor">
          <div class="rule-editor__header">
            <div>
              <h2>Rule Editor</h2>
              <p class="text-muted">監視条件を編集し、設定ファイルへ保存します。</p>
            </div>
            <div class="button-row">
              <button class="button button-secondary" type="button" :disabled="!selectedRule" @click="removeSelectedRule">
                Delete
              </button>
              <button class="button button-primary" type="button" :disabled="saveState === 'saving'" @click="persistRules">
                {{ saveState === 'saving' ? 'Saving...' : 'Save Rules' }}
              </button>
            </div>
          </div>

          <template v-if="selectedRule">
            <div class="form-grid">
              <label class="field">
                <span class="field__label">Rule ID</span>
                <input v-model="selectedRule.id" class="field__control" type="text" />
              </label>

              <label class="field">
                <span class="field__label">Name</span>
                <input v-model="selectedRule.name" class="field__control" type="text" />
              </label>

              <label class="field">
                <span class="field__label">Target</span>
                <select v-model="selectedRule.target" class="field__control">
                  <option value="issue">Issue</option>
                  <option value="pull_request">Pull Request</option>
                </select>
              </label>

              <label class="field field-checkbox">
                <input v-model="selectedRule.enabled" type="checkbox" />
                <span>Enabled</span>
              </label>

              <label class="field field-full">
                <span class="field__label">Repositories</span>
                <input
                  v-model="selectedRule.repositoriesText"
                  class="field__control"
                  type="text"
                  placeholder="owner/repo-a, owner/repo-b"
                />
              </label>

              <label class="field field-full">
                <span class="field__label">Labels</span>
                <input v-model="selectedRule.labelsText" class="field__control" type="text" placeholder="ai:design, backend" />
              </label>

              <label class="field field-full">
                <span class="field__label">Title Pattern</span>
                <input v-model="selectedRule.titlePattern" class="field__control" type="text" placeholder="^feat:" />
              </label>

              <label class="field">
                <span class="field__label">Authors</span>
                <input v-model="selectedRule.authorsText" class="field__control" type="text" placeholder="alice, bob" />
              </label>

              <label class="field">
                <span class="field__label">Assignees</span>
                <input v-model="selectedRule.assigneesText" class="field__control" type="text" placeholder="carol, dave" />
              </label>

              <label class="field">
                <span class="field__label">Skill Set</span>
                <select v-model="selectedRule.skillSet" class="field__control">
                  <option v-for="skillSet in skillSets ?? []" :key="skillSet.name" :value="skillSet.name">
                    {{ skillSet.name }}
                  </option>
                </select>
              </label>

              <label class="field">
                <span class="field__label">Test Profile</span>
                <input v-model="selectedRule.testProfile" class="field__control" type="text" />
              </label>

              <label class="field">
                <span class="field__label">Provider</span>
                <select v-model="selectedRule.provider" class="field__control">
                  <option v-for="option in watchRuleProviderOptions(appConfig?.providers ?? [])" :key="option.value" :value="option.value">
                    {{ option.label }}
                  </option>
                </select>
              </label>

              <label class="field">
                <span class="field__label">Model</span>
                <select v-model="selectedRule.model" class="field__control">
                  <option v-for="option in availableModelOptions" :key="option.value" :value="option.value">
                    {{ option.label }}
                  </option>
                </select>
              </label>

              <label class="field field-checkbox field-full">
                <input v-model="selectedRule.excludeDraftPR" type="checkbox" />
                <span>Exclude Draft Pull Requests</span>
              </label>
            </div>

            <div v-if="saveState === 'saved'" class="notice notice-success">watch-rules.yaml を更新しました。</div>
            <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
          </template>

          <div v-else class="notice">Rule を追加するか、左側の一覧から選択してください。</div>
        </section>
      </section>
    </AsyncState>
  </AppShell>
</template>
