import { createRouter, createWebHistory } from 'vue-router'
import DashboardPage from './pages/DashboardPage.vue'
import GuidePage from './pages/GuidePage.vue'
import ImprovementsPage from './pages/ImprovementsPage.vue'
import JobDetailPage from './pages/JobDetailPage.vue'
import SettingsPage from './pages/SettingsPage.vue'
import SkillSetsPage from './pages/SkillSetsPage.vue'
import TestProfilesPage from './pages/TestProfilesPage.vue'
import ToolCommandsPage from './pages/ToolCommandsPage.vue'
import WorkerSettingsPage from './pages/WorkerSettingsPage.vue'
import WatchRulesPage from './pages/WatchRulesPage.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardPage },
    { path: '/guide', name: 'guide', component: GuidePage },
    { path: '/improvements', name: 'improvements', component: ImprovementsPage },
    { path: '/jobs/:id', name: 'job-detail', component: JobDetailPage, props: true },
    { path: '/settings', name: 'settings', component: SettingsPage },
    { path: '/settings/workers', name: 'workers', component: WorkerSettingsPage },
    { path: '/settings/test-profiles', name: 'test-profiles', component: TestProfilesPage },
    { path: '/settings/tool-commands', name: 'tool-commands', component: ToolCommandsPage },
    { path: '/settings/watch-rules', name: 'watch-rules', component: WatchRulesPage },
    { path: '/settings/skillsets', name: 'skillsets', component: SkillSetsPage },
  ],
})
