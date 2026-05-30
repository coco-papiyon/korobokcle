<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import guideMarkdown from '@/content/guide.md?raw'
import { computed } from 'vue'

function escapeHTML(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
}

function renderMarkdown(value: string): string {
  const lines = value.replace(/\r\n/g, '\n').split('\n')
  const out: string[] = []
  let inList = false
  let inCode = false
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
      out.push(`<h2>${escapeHTML(trimmed.slice(3))}</h2>`)
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

const headings = computed(() =>
  guideMarkdown
    .replace(/\r\n/g, '\n')
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.startsWith('## '))
    .map((line) => {
      const title = line.slice(3).trim()
      const slug = title
        .toLowerCase()
        .replace(/[^a-z0-9\s-]/g, '')
        .trim()
        .replace(/\s+/g, '-')
      return { title, slug }
    }),
)

function renderMarkdownWithAnchors(value: string): string {
  return renderMarkdown(value).replaceAll(
    /<h2>(.*?)<\/h2>/g,
    (_, title: string) => {
      const slug = String(title)
        .toLowerCase()
        .replace(/[^a-z0-9\s-]/g, '')
        .trim()
        .replace(/\s+/g, '-')
      return `<h2 id="${slug}">${title}</h2>`
    },
  )
}
</script>

<template>
  <AppShell
    title="Guide"
    description="基本的な使い方を確認できます。"
  >
    <section class="guide-layout">
      <aside class="panel guide-toc">
        <h2>Contents</h2>
        <nav class="stack-sm" aria-label="Guide contents">
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
