<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { createSkillSet, deleteSkillSet, fetchSkillSet, fetchSkillSets, saveSkillSet } from '@/lib/api'
import type { SkillFile, SkillSet, SkillSetSummary } from '@/types'

const skillOrder = ['design', 'implement', 'fix', 'review'] as const

const { data, isLoading, error, reload } = useAsyncData(fetchSkillSets)
const selectedName = ref('default')
const selectedSet = ref<SkillSet | null>(null)
const detailLoading = ref(false)
const detailError = ref<string | null>(null)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveError = ref<string | null>(null)
const createName = ref('')
const createSource = ref('default')
const createState = ref<'idle' | 'creating' | 'error'>('idle')
const createError = ref<string | null>(null)

const skillSetOptions = computed(() => data.value ?? [])

watch(
  skillSetOptions,
  async (sets) => {
    if (!sets.length) {
      selectedSet.value = null
      return
    }
    if (!sets.some((set) => set.name === selectedName.value)) {
      selectedName.value = sets[0]?.name ?? 'default'
    }
    await loadSkillSet(selectedName.value)
  },
  { immediate: true },
)

async function loadSkillSet(name: string) {
  detailLoading.value = true
  detailError.value = null
  saveState.value = 'idle'
  saveError.value = null
  try {
    selectedSet.value = cloneSkillSet(await fetchSkillSet(name))
  } catch (err) {
    detailError.value = err instanceof Error ? err.message : 'Unknown error'
    selectedSet.value = null
  } finally {
    detailLoading.value = false
  }
}

function cloneSkillFile(file: SkillFile): SkillFile {
  return {
    definition: {
      ...file.definition,
      inputs: [...(file.definition.inputs ?? [])],
      outputs: [...(file.definition.outputs ?? [])],
      artifacts: { ...file.definition.artifacts },
    },
    promptTemplate: file.promptTemplate,
  }
}

function cloneSkillSet(set: SkillSet): SkillSet {
  const skills: Record<string, SkillFile> = {}
  Object.entries(set.skills).forEach(([name, file]) => {
    skills[name] = cloneSkillFile(file)
  })
  return {
    name: set.name,
    mutable: set.mutable,
    skills,
  }
}

async function selectSkillSet(name: string) {
  selectedName.value = name
  await loadSkillSet(name)
}

async function persistSkillSet() {
  if (!selectedSet.value?.mutable) {
    return
  }
  saveState.value = 'saving'
  saveError.value = null
  try {
    const saved = await saveSkillSet(selectedSet.value)
    selectedSet.value = cloneSkillSet(saved)
    saveState.value = 'saved'
    await reload()
  } catch (err) {
    saveState.value = 'error'
    saveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function createNewSkillSet() {
  createState.value = 'creating'
  createError.value = null
  try {
    const created = await createSkillSet(createName.value.trim(), createSource.value)
    createName.value = ''
    await reload()
    selectedName.value = created.name
    selectedSet.value = cloneSkillSet(created)
    saveState.value = 'idle'
  } catch (err) {
    createState.value = 'error'
    createError.value = err instanceof Error ? err.message : 'Unknown error'
    return
  }
  createState.value = 'idle'
}

async function removeSelectedSkillSet() {
  if (!selectedSet.value?.mutable) {
    return
  }
  await deleteSkillSet(selectedSet.value.name)
  await reload()
}

function skillLabel(name: string) {
  switch (name) {
    case 'design':
      return 'Design'
    case 'implement':
      return 'Implement'
    case 'fix':
      return 'Fix'
    case 'review':
      return 'Review'
    default:
      return name
  }
}
</script>

<template>
  <AppShell
    title="Skill Sets"
    description="自動処理を実行するスキルを設定します。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <section class="watch-layout">
        <aside class="panel rule-list">
          <div class="rule-list__header">
            <div>
              <h2>Skill Sets</h2>
              <p class="text-muted">default を基点に複製し、用途ごとに編集します。</p>
            </div>
          </div>

          <div class="stack-sm">
            <button
              v-for="set in skillSetOptions"
              :key="set.name"
              class="rule-item"
              :class="{ 'rule-item--active': selectedName === set.name }"
              type="button"
              @click="selectSkillSet(set.name)"
            >
              <div class="rule-item__head">
                <strong>{{ set.name }}</strong>
                <span class="text-muted">{{ set.mutable ? 'custom' : 'read-only' }}</span>
              </div>
            </button>
          </div>

          <div class="stack-sm skillset-create">
            <h3>Create / Copy</h3>
            <label class="field">
              <span class="field__label">Name</span>
              <input v-model="createName" class="field__control" type="text" placeholder="team-a" />
            </label>
            <label class="field">
              <span class="field__label">Source</span>
              <select v-model="createSource" class="field__control">
                <option v-for="set in skillSetOptions" :key="`source-${set.name}`" :value="set.name">{{ set.name }}</option>
              </select>
            </label>
            <button class="button button-primary" type="button" :disabled="createState === 'creating'" @click="createNewSkillSet">
              {{ createState === 'creating' ? 'Creating...' : 'Create Skill Set' }}
            </button>
            <div v-if="createState === 'error'" class="notice notice-danger">{{ createError }}</div>
          </div>
        </aside>

        <section class="panel rule-editor">
          <div class="rule-editor__header">
            <div>
              <h2>Skill Set Editor</h2>
              <p class="text-muted">
                {{ selectedSet?.mutable ? 'provider と prompt を編集して保存できます。' : 'default は編集不可です。複製して変更してください。' }}
              </p>
            </div>
            <div class="button-row">
              <button
                class="button button-secondary"
                type="button"
                :disabled="!selectedSet?.mutable"
                @click="removeSelectedSkillSet"
              >
                Delete
              </button>
              <button class="button button-primary" type="button" :disabled="saveState === 'saving' || !selectedSet?.mutable" @click="persistSkillSet">
                {{ saveState === 'saving' ? 'Saving...' : 'Save Skill Set' }}
              </button>
            </div>
          </div>

          <div v-if="detailLoading" class="notice">Loading skill set...</div>
          <div v-else-if="detailError" class="notice notice-danger">{{ detailError }}</div>
          <template v-else-if="selectedSet">
            <section v-for="skillName in skillOrder" :key="skillName" class="panel stack-md skill-panel">
              <div>
                <h3>{{ skillLabel(skillName) }}</h3>
                <p class="text-muted">
                  Inputs: {{ selectedSet.skills[skillName]?.definition.inputs.join(', ') || 'none' }} / Outputs:
                  {{ selectedSet.skills[skillName]?.definition.outputs.join(', ') || 'none' }}
                </p>
              </div>

              <div class="form-grid">
                <label class="field">
                  <span class="field__label">Provider</span>
                  <input v-model="selectedSet.skills[skillName].definition.provider" class="field__control" type="text" :disabled="!selectedSet.mutable" />
                </label>

                <label class="field">
                  <span class="field__label">Output File</span>
                  <input
                    v-model="selectedSet.skills[skillName].definition.artifacts.output_file"
                    class="field__control"
                    type="text"
                    :disabled="!selectedSet.mutable"
                  />
                </label>

                <label class="field field-full">
                  <span class="field__label">Prompt Template</span>
                  <textarea
                    v-model="selectedSet.skills[skillName].promptTemplate"
                    class="field__control field__control--textarea"
                    rows="18"
                    :disabled="!selectedSet.mutable"
                  />
                </label>
              </div>
            </section>

            <div v-if="saveState === 'saved'" class="notice notice-success">skill set を保存しました。</div>
            <div v-if="saveState === 'error'" class="notice notice-danger">{{ saveError }}</div>
          </template>
        </section>
      </section>
    </AsyncState>
  </AppShell>
</template>
