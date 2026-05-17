import { createRouter, createWebHistory } from 'vue-router'
import DashboardPage from './pages/DashboardPage.vue'
import JobDetailPage from './pages/JobDetailPage.vue'
import SettingsPage from './pages/SettingsPage.vue'
import SkillSetsPage from './pages/SkillSetsPage.vue'
import WatchRulesPage from './pages/WatchRulesPage.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardPage },
    { path: '/jobs/:id', name: 'job-detail', component: JobDetailPage, props: true },
    { path: '/settings', name: 'settings', component: SettingsPage },
    { path: '/settings/watch-rules', name: 'watch-rules', component: WatchRulesPage },
    { path: '/settings/skillsets', name: 'skillsets', component: SkillSetsPage },
  ],
})
