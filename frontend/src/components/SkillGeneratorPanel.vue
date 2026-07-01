<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { AIProvider, SkillGenerationResult, SkillStatus, WatchSettings } from '../types'

const formStorageKey = 'korobokcle.skillGenerationForm.v1'

const skills = ref<SkillStatus[]>([])
const selectedPurposes = ref<string[]>([])
const provider = ref<AIProvider>('codex')
const loading = ref(false)
const generating = ref(false)
const error = ref('')
const message = ref('')
const projectContext = ref('')
const testCommand = ref('go test ./...')
const maxFixLoops = ref(3)

const selectedCount = computed(() => selectedPurposes.value.length)
const allSelected = computed({
  get: () => skills.value.length > 0 && selectedPurposes.value.length === skills.value.length,
  set: (checked: boolean) => {
    selectedPurposes.value = checked ? skills.value.map((skill) => skill.purpose) : []
  },
})

function providerLabel(value: AIProvider) {
  return value === 'github_copilot' ? 'GitHub Copilot' : 'Codex'
}

function statusLabel(skill: SkillStatus) {
  if (skill.generated) return 'AI生成済み'
  if (skill.aiExists) return 'AI確認済み'
  if (skill.exists) return 'ローカル存在'
  return '未生成'
}

function statusChipClass(skill: SkillStatus) {
  return {
    'chip--simple': skill.exists && !skill.aiExists && !skill.generated,
    'chip--ai': skill.aiExists && !skill.generated,
    'chip--generated': skill.generated,
    'chip--missing': !skill.exists && !skill.aiExists && !skill.generated,
  }
}

function restoreSavedForm() {
  const raw = window.localStorage.getItem(formStorageKey)
  if (!raw) return
  try {
    const saved = JSON.parse(raw) as {
      projectContext?: string
      testCommand?: string
      maxFixLoops?: number
    }
    projectContext.value = typeof saved.projectContext === 'string' ? saved.projectContext : ''
    testCommand.value = typeof saved.testCommand === 'string' && saved.testCommand.length > 0 ? saved.testCommand : 'go test ./...'
    maxFixLoops.value = typeof saved.maxFixLoops === 'number' && Number.isFinite(saved.maxFixLoops) ? saved.maxFixLoops : 3
  } catch {
    window.localStorage.removeItem(formStorageKey)
  }
}

function persistForm() {
  window.localStorage.setItem(
    formStorageKey,
    JSON.stringify({
      projectContext: projectContext.value,
      testCommand: testCommand.value,
      maxFixLoops: maxFixLoops.value,
    }),
  )
}

async function loadSkills() {
  loading.value = true
  error.value = ''
  try {
    const [skillsResponse, settingsResponse] = await Promise.all([
      fetch('/api/skills', { cache: 'no-store' }),
      fetch('/api/settings', { cache: 'no-store' }),
    ])
    if (!skillsResponse.ok) throw new Error((await skillsResponse.text()) || `HTTP ${skillsResponse.status}`)
    if (!settingsResponse.ok) throw new Error((await settingsResponse.text()) || `HTTP ${settingsResponse.status}`)
    const skillPayload = (await skillsResponse.json()) as { skills?: SkillStatus[] }
    const settings = (await settingsResponse.json()) as WatchSettings
    skills.value = skillPayload.skills ?? []
    selectedPurposes.value = skills.value.map((skill) => skill.purpose)
    provider.value = settings.aiProvider ?? 'codex'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    loading.value = false
  }
}

async function submitSkillsGeneration(overwriteExisting = false) {
  const forcePurposes = selectedPurposes.value.filter((purpose) =>
    skills.value.some((skill) => skill.purpose === purpose),
  )
  generating.value = true
  error.value = ''
  message.value = ''
  try {
    const response = await fetch('/api/skills', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        projectContext: projectContext.value.trim(),
        testCommand: testCommand.value.trim(),
        maxFixLoops: maxFixLoops.value,
        forcePurposes,
        overwriteExisting,
      }),
    })
    if (!response.ok) throw new Error((await response.text()) || `HTTP ${response.status}`)
    const result = (await response.json()) as SkillGenerationResult
    skills.value = result.skills
    provider.value = result.provider
    message.value = result.message
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    generating.value = false
  }
}

function generateSelectedSkills() {
  return submitSkillsGeneration(false)
}

function regenerateSelectedSkills() {
  return submitSkillsGeneration(true)
}

onMounted(() => {
  restoreSavedForm()
  void loadSkills()
})

watch([projectContext, testCommand, maxFixLoops], () => {
  persistForm()
})

defineExpose({
  loadSkills,
  generateSelectedSkills,
  regenerateSelectedSkills,
  loading,
  generating,
  selectedCount,
})
</script>

<template>
  <p v-if="error" class="error">{{ error }}</p>
  <p v-if="message" class="success">{{ message }}</p>

  <div class="skill-layout">
    <section class="settings-section">
      <h3>生成情報</h3>
      <label class="field">
        <span>プロジェクト固有情報</span>
        <textarea
          v-model="projectContext"
          class="control artifact-comment"
          rows="6"
          placeholder="使用言語、フレームワーク、設計規約、確認必須事項など"
        ></textarea>
      </label>
      <label class="field">
        <span>テストコマンド</span>
        <textarea
          v-model="testCommand"
          class="control artifact-comment"
          rows="4"
          placeholder="go test ./...\ngo test ./internal/app"
        ></textarea>
      </label>
      <label class="field">
        <span>テスト失敗時の再修正上限</span>
        <input v-model.number="maxFixLoops" class="control" type="number" min="1" max="20" step="1" />
      </label>
      <p class="field-note">実装スキルはテストと再修正をこの回数まで繰り返す。設計スキルにはテスト実装方針を含める。</p>
    </section>

    <section>
      <div class="panel__title-row">
        <h2>スキル状態</h2>
        <label class="skill-select-all">
          <input v-model="allSelected" type="checkbox" />
          <span>全選択</span>
        </label>
        <span class="panel__hint">{{ selectedCount }} / {{ skills.length }} 件 選択</span>
      </div>
      <div v-if="loading" class="empty-state">確認中...</div>
      <div v-else class="skill-status-list">
        <article v-for="skill in skills" :key="skill.purpose" class="skill-status-card">
          <div>
            <h3>{{ skill.displayName }}</h3>
            <code>{{ skill.name }}</code>
            <p v-if="skill.path" class="skill-status-card__path">{{ skill.path }}</p>
          </div>
          <div class="skill-status-card__actions">
            <label class="skill-status-card__select">
              <input v-model="selectedPurposes" type="checkbox" :value="skill.purpose" />
              <span>対象</span>
            </label>
            <span class="chip" :class="statusChipClass(skill)">
              {{ statusLabel(skill) }}
            </span>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>
