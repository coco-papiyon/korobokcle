<script setup lang="ts">
import { ref } from 'vue'
import SettingsPanel from './components/SettingsPanel.vue'
import JobListPanel from './components/JobListPanel.vue'
import JobDetailPanel from './components/JobDetailPanel.vue'
import SkillGeneratorPanel from './components/SkillGeneratorPanel.vue'

type Tab = 'settings' | 'skills' | 'jobs' | 'detail'

const activeTab = ref<Tab>('jobs')
const selectedJobId = ref('')
const detailRefreshKey = ref(0)
const tabDescriptions: Record<Tab, string> = {
  settings: 'AI プロバイダーと監視条件をまとめて設定する。',
  skills: 'Issue駆動開発に必要な Agent Skill を監視対象リポジトリへ生成する。',
  jobs: '監視中のジョブ一覧を確認し、処理対象を選択する。',
  detail: '選択したジョブの詳細と生成物を確認し、必要なら再実行や承認を行う。',
}

function selectJob(jobId: string) {
  selectedJobId.value = jobId
  detailRefreshKey.value += 1
  activeTab.value = 'detail'
}

function selectTab(tab: Tab) {
  activeTab.value = tab
}
</script>

<template>
  <div class="app-shell">
    <main class="dashboard">
      <section class="panel">
        <div class="tabs" role="tablist" aria-label="korobokcle views">
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

        <p class="tab-description" aria-live="polite">
          {{ tabDescriptions[activeTab] }}
        </p>

        <div v-show="activeTab === 'settings'" class="tab-panel" role="tabpanel">
          <SettingsPanel />
        </div>

        <div v-show="activeTab === 'jobs'" class="tab-panel" role="tabpanel">
          <JobListPanel :selected-job-id="selectedJobId" :active="activeTab === 'jobs'" @select="selectJob" />
        </div>

        <div v-show="activeTab === 'skills'" class="tab-panel" role="tabpanel">
          <SkillGeneratorPanel />
        </div>

        <div v-show="activeTab === 'detail'" class="tab-panel" role="tabpanel">
          <JobDetailPanel :job-id="selectedJobId" :refresh-key="detailRefreshKey" :active="activeTab === 'detail'" />
        </div>
      </section>
    </main>
  </div>
</template>
