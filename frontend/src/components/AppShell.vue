<script setup lang="ts">
defineProps<{
  title: string
  description: string
}>()
</script>

<template>
  <div class="app-shell">
    <header class="hero panel panel-hero">
      <nav class="nav-tabs" aria-label="Main">
        <RouterLink class="nav-tabs__link" to="/">ダッシュボード</RouterLink>
        <RouterLink class="nav-tabs__link" to="/settings">設定</RouterLink>
        <RouterLink class="nav-tabs__link" to="/settings/workers">ワーカー</RouterLink>
        <RouterLink class="nav-tabs__link" to="/settings/watch-rules">監視ルール</RouterLink>
        <RouterLink class="nav-tabs__link" to="/settings/skillsets">スキルセット</RouterLink>
        <RouterLink class="nav-tabs__link" to="/settings/test-profiles">テストプロファイル</RouterLink>
        <RouterLink class="nav-tabs__link" to="/settings/tool-commands">ツールコマンド</RouterLink>
        <RouterLink class="nav-tabs__link" to="/improvements">改善一覧</RouterLink>
        <RouterLink class="nav-tabs__link" to="/guide">ガイド</RouterLink>
      </nav>
      <p class="hero__description">{{ description }}</p>
    </header>
    <main class="stack-lg">
      <slot />
    </main>
  </div>
</template>
