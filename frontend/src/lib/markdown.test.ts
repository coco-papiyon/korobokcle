import { describe, expect, it } from 'vitest'

import { EMPTY_ISSUE_BODY_MESSAGE } from '@/lib/ui-text'
import { renderMarkdown } from './markdown'
import { formatIssueBody } from './format'

describe('markdown rendering', () => {
  it('renders headings, tables, links and code blocks safely', () => {
    const html = renderMarkdown(
      [
        '# Title',
        '',
        '## Section',
        '',
        '| Name | Value |',
        '| --- | --- |',
        '| Example | [OpenAI](https://openai.com) |',
        '| Escaped pipe | a\\|b |',
        '| Code span | `left | right` |',
        '| Unsafe | [Bad](javascript:alert(1)) |',
        '',
        '- first',
        '- second',
        '',
        '```ts',
        'const value = "<tag>"',
        '```',
        '',
        '<script>alert(1)</script>',
      ].join('\n'),
      { headingIds: ['section'] },
    )

    expect(html).toContain('<h1 id="title">Title</h1>')
    expect(html).toContain('<h2 id="section">Section</h2>')
    expect(html).toContain('<div class="markdown-table"><table>')
    expect(html).toContain('<a href="https://openai.com">OpenAI</a>')
    expect(html).toContain('<td>a|b</td>')
    expect(html).toContain('<td><code>left | right</code></td>')
    expect(html).not.toContain('href="#"')
    expect(html).not.toContain('<a href="javascript:alert(1)">Bad</a>')
    expect(html).toContain('<ul><li>first</li><li>second</li></ul>')
    expect(html).toContain('<pre><code class="language-ts">const value = &quot;&lt;tag&gt;&quot;</code></pre>')
    expect(html).toContain('&lt;script&gt;alert(1)&lt;/script&gt;')
    expect(html).not.toContain('<script>alert(1)</script>')
  })

  it('falls back to the empty issue body message', () => {
    expect(formatIssueBody('')).toBe(EMPTY_ISSUE_BODY_MESSAGE)
    expect(formatIssueBody('   ')).toBe(EMPTY_ISSUE_BODY_MESSAGE)
    expect(renderMarkdown(formatIssueBody(''))).toContain(EMPTY_ISSUE_BODY_MESSAGE)
  })
})
