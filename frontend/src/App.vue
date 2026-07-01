<script setup lang="ts">
import { ref } from 'vue'
import SettingsPanel from './components/SettingsPanel.vue'
import JobListPanel from './components/JobListPanel.vue'
import JobDetailModal from './components/JobDetailModal.vue'
import SkillGeneratorPanel from './components/SkillGeneratorPanel.vue'

type Tab = 'settings' | 'skills' | 'jobs'

const activeTab = ref<Tab>('jobs')
const selectedJobId = ref('')
const isJobDetailOpen = ref(false)
const jobListRefreshKey = ref(0)
const detailRefreshKey = ref(0)
const settingsPanelRef = ref<InstanceType<typeof SettingsPanel> | null>(null)
const skillPanelRef = ref<InstanceType<typeof SkillGeneratorPanel> | null>(null)
const tabDescriptions: Record<Tab, string> = {
  settings: 'AI プロバイダーと監視条件をまとめて設定する。',
  skills: 'Issue駆動開発に必要な Agent Skill を監視対象リポジトリへ生成する。',
  jobs: '監視中のジョブ一覧を確認し、処理対象を選択する。',
}

function selectJob(jobId: string) {
  selectedJobId.value = jobId
  isJobDetailOpen.value = true
  detailRefreshKey.value += 1
}

function closeJobDetail() {
  isJobDetailOpen.value = false
}

function refreshJobs() {
  jobListRefreshKey.value += 1
}

function handleJobDeleted(jobId: string) {
  if (selectedJobId.value === jobId) {
    selectedJobId.value = ''
  }
  refreshJobs()
  closeJobDetail()
}

function handleJobDetailRefresh() {
  refreshJobs()
}

function selectTab(tab: Tab) {
  activeTab.value = tab
}

function saveSettings() {
  void settingsPanelRef.value?.saveSettings()
}

function refreshSkills() {
  void skillPanelRef.value?.loadSkills()
}

function generateSkills() {
  void skillPanelRef.value?.generateSelectedSkills()
}

function regenerateSkills() {
  void skillPanelRef.value?.regenerateSelectedSkills()
}
</script>

<template>
  <div class="app-shell">
    <main class="dashboard">
      <section class="panel">
        <div class="tabs" role="tablist" aria-label="korobokcle views">
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
            :class="{ 'tab--active': activeTab === 'skills' }"
            type="button"
            role="tab"
            :aria-selected="activeTab === 'skills'"
            @click="selectTab('skills')"
          >
            スキル生成
          </button>
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
        </div>

        <div class="tab-description" aria-live="polite">
          <span class="tab-description__text">{{ tabDescriptions[activeTab] }}</span>
          <div class="tab-description__actions">
            <button
              v-if="activeTab === 'settings'"
              class="button button--small"
              type="button"
              :disabled="settingsPanelRef?.settingsSaving"
              @click="saveSettings"
            >
              {{ settingsPanelRef?.settingsSaving ? '保存中' : '保存' }}
            </button>
            <template v-else-if="activeTab === 'skills'">
              <button
                class="button button--ghost button--small"
                type="button"
                :disabled="skillPanelRef?.loading || skillPanelRef?.generating"
                @click="refreshSkills"
              >
                {{ skillPanelRef?.loading ? '確認中' : '再確認' }}
              </button>
              <button
                class="button button--small"
                type="button"
                :disabled="skillPanelRef?.generating || (skillPanelRef?.selectedCount ?? 0) === 0"
                @click="generateSkills"
              >
                {{ skillPanelRef?.generating ? 'AIで生成中' : `選択スキルを生成 (${skillPanelRef?.selectedCount ?? 0})` }}
              </button>
              <button
                class="button button--ghost button--small"
                type="button"
                :disabled="skillPanelRef?.generating || (skillPanelRef?.selectedCount ?? 0) === 0"
                @click="regenerateSkills"
              >
                {{ skillPanelRef?.generating ? '再生成中' : `選択スキルを再生成[上書き] (${skillPanelRef?.selectedCount ?? 0})` }}
              </button>
            </template>
          </div>
        </div>

        <div v-show="activeTab === 'jobs'" class="tab-panel" role="tabpanel">
          <JobListPanel
            :active="activeTab === 'jobs'"
            :selected-job-id="selectedJobId"
            :refresh-key="jobListRefreshKey"
            @select="selectJob"
          />
        </div>

        <div v-show="activeTab === 'skills'" class="tab-panel" role="tabpanel">
          <SkillGeneratorPanel ref="skillPanelRef" />
        </div>

        <div v-show="activeTab === 'settings'" class="tab-panel" role="tabpanel">
          <SettingsPanel ref="settingsPanelRef" />
        </div>
      </section>
    </main>

    <JobDetailModal
      v-if="isJobDetailOpen"
      :job-id="selectedJobId"
      :refresh-key="detailRefreshKey"
      @close="closeJobDetail"
      @deleted="handleJobDeleted"
      @refresh="handleJobDetailRefresh"
    />
  </div>
</template>
