<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import guideMarkdown from '@/content/guide.md?raw'
import { computed } from 'vue'
import { renderMarkdown } from '@/lib/markdown'

type GuideHeading = {
  title: string
  slug: string
}

const guideHeadings: GuideHeading[] = [
  { title: '概要', slug: 'overview' },
  { title: '基本フロー', slug: 'basic-flow' },
  { title: 'ツールコマンド', slug: 'tool-commands' },
  { title: '注意事項', slug: 'notes' },
  { title: 'インストール', slug: 'installation' },
]

const headings = computed(() => guideHeadings)
const guideMarkdownHtml = computed(() =>
  renderMarkdown(guideMarkdown, {
    headingIds: guideHeadings.map((heading) => heading.slug),
  }),
)
</script>

<template>
  <AppShell
    title="ガイド"
    description="基本的な使い方を確認できます。"
  >
    <section class="guide-layout">
      <aside class="panel guide-toc">
        <h2>目次</h2>
        <nav class="stack-sm" aria-label="ガイドの目次">
          <a
            v-for="heading in headings"
            :key="heading.slug"
            class="guide-toc__link"
            :href="`#${heading.slug}`"
          >
            {{ heading.title }}
          </a>
        </nav>
      </aside>
      <section class="panel markdown-content" v-html="guideMarkdownHtml" />
    </section>
  </AppShell>
</template>
