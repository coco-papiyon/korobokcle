<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import guideMarkdown from '@/content/guide.md?raw'
import { computed } from 'vue'

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

function escapeHTML(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
}

function renderMarkdown(value: string, headingSlugs: string[] = []): string {
  const lines = value.replace(/\r\n/g, '\n').split('\n')
  const out: string[] = []
  let inList = false
  let inCode = false
  let headingIndex = 0
  const codeLines: string[] = []

  function closeList() {
    if (inList) {
      out.push('</ul>')
      inList = false
    }
  }

  function closeCode() {
    if (inCode) {
      out.push(`<pre><code>${escapeHTML(codeLines.join('\n'))}</code></pre>`)
      codeLines.length = 0
      inCode = false
    }
  }

  for (const rawLine of lines) {
    const line = rawLine.trimEnd()
    const trimmed = line.trim()

    if (trimmed.startsWith('```')) {
      closeList()
      if (inCode) {
        closeCode()
      } else {
        inCode = true
      }
      continue
    }

    if (inCode) {
      codeLines.push(line)
      continue
    }

    if (trimmed === '') {
      closeList()
      continue
    }

    if (trimmed.startsWith('# ')) {
      closeList()
      out.push(`<h1>${escapeHTML(trimmed.slice(2))}</h1>`)
      continue
    }
    if (trimmed.startsWith('## ')) {
      closeList()
      const title = escapeHTML(trimmed.slice(3))
      const slug = headingSlugs[headingIndex++] ?? `heading-${headingIndex}`
      out.push(`<h2 id="${slug}">${title}</h2>`)
      continue
    }
    if (trimmed.startsWith('### ')) {
      closeList()
      out.push(`<h3>${escapeHTML(trimmed.slice(4))}</h3>`)
      continue
    }
    if (trimmed.startsWith('- ')) {
      if (!inList) {
        out.push('<ul>')
        inList = true
      }
      out.push(`<li>${escapeHTML(trimmed.slice(2))}</li>`)
      continue
    }
    if (/^\d+\.\s/.test(trimmed)) {
      closeList()
      out.push(`<p>${escapeHTML(trimmed)}</p>`)
      continue
    }

    closeList()
    out.push(`<p>${escapeHTML(trimmed)}</p>`)
  }

  closeCode()
  closeList()
  return out.join('\n')
}

const headings = computed(() => guideHeadings)

function renderMarkdownWithAnchors(value: string): string {
  return renderMarkdown(
    value,
    guideHeadings.map((heading) => heading.slug),
  )
}
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
      <section class="panel markdown-content" v-html="renderMarkdownWithAnchors(guideMarkdown)" />
    </section>
  </AppShell>
</template>
