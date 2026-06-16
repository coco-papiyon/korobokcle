export type MarkdownRenderOptions = {
  headingIds?: string[]
}

function escapeHTML(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function slugify(value: string): string {
  const normalized = value
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[^\p{Letter}\p{Number}]+/gu, '-')
    .replace(/^-+|-+$/g, '')
  return normalized || 'section'
}

function sanitizeHref(value: string): string {
  const href = value.trim()
  if (
    href.startsWith('http://') ||
    href.startsWith('https://') ||
    href.startsWith('mailto:') ||
    href.startsWith('tel:') ||
    href.startsWith('#') ||
    (href.startsWith('/') && !href.startsWith('//')) ||
    href.startsWith('?') ||
    href.startsWith('./') ||
    href.startsWith('../')
  ) {
    return href
  }
  return '#'
}

function renderInlineMarkdown(value: string): string {
  let output = ''
  let index = 0

  while (index < value.length) {
    const current = value[index]

    if (current === '`') {
      const closing = value.indexOf('`', index + 1)
      if (closing !== -1) {
        output += `<code>${escapeHTML(value.slice(index + 1, closing))}</code>`
        index = closing + 1
        continue
      }
    }

    if (current === '[') {
      const closingLabel = value.indexOf(']', index + 1)
      const openingHref = closingLabel !== -1 ? value[closingLabel + 1] : ''
      if (closingLabel !== -1 && openingHref === '(') {
        let closingHref = closingLabel + 2
        let depth = 1
        while (closingHref < value.length) {
          const char = value[closingHref]
          if (char === '(') {
            depth += 1
          } else if (char === ')') {
            depth -= 1
            if (depth === 0) {
              break
            }
          }
          closingHref += 1
        }

        if (depth === 0) {
          const label = value.slice(index + 1, closingLabel)
          const href = value.slice(closingLabel + 2, closingHref)
          output += `<a href="${escapeHTML(sanitizeHref(href))}">${renderInlineMarkdown(label)}</a>`
          index = closingHref + 1
          continue
        }
      }
    }

    output += escapeHTML(current)
    index += 1
  }

  return output
}

function isHeadingLine(line: string): RegExpMatchArray | null {
  return line.match(/^(#{1,6})\s+(.+)$/)
}

function isFenceLine(line: string): RegExpMatchArray | null {
  return line.match(/^```([\w-]+)?\s*$/)
}

function isBulletLine(line: string): RegExpMatchArray | null {
  return line.match(/^\s*[-*+]\s+(.+)$/)
}

function isOrderedLine(line: string): RegExpMatchArray | null {
  return line.match(/^\s*(\d+)\.\s+(.+)$/)
}

function isTableSeparatorLine(line: string): boolean {
  const trimmed = line.trim()
  if (!trimmed.includes('|')) {
    return false
  }
  const cells = splitTableRow(trimmed)
  return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/.test(cell.trim()))
}

function splitTableRow(line: string): string[] {
  const trimmed = line.trim().replace(/^\|/, '').replace(/\|$/, '')
  return trimmed.split('|').map((cell) => cell.trim())
}

function isTableRow(line: string): boolean {
  return line.trim().includes('|')
}

function isBlockBoundary(line: string): boolean {
  const trimmed = line.trim()
  return (
    trimmed === '' ||
    Boolean(isHeadingLine(trimmed)) ||
    Boolean(isFenceLine(trimmed)) ||
    Boolean(isBulletLine(trimmed)) ||
    Boolean(isOrderedLine(trimmed)) ||
    Boolean(isTableSeparatorLine(trimmed))
  )
}

function renderTable(headerLine: string, rows: string[]): string {
  const headers = splitTableRow(headerLine)
  const bodyRows = rows.map((row) => splitTableRow(row))
  const head = headers.map((cell) => `<th>${renderInlineMarkdown(cell)}</th>`).join('')
  const body = bodyRows
    .map((row) => {
      const cells = headers.map((_, index) => `<td>${renderInlineMarkdown(row[index] ?? '')}</td>`).join('')
      return `<tr>${cells}</tr>`
    })
    .join('')

  return `<div class="markdown-table"><table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table></div>`
}

function renderList(items: string[], ordered: boolean, start: number): string {
  const tag = ordered ? 'ol' : 'ul'
  const startAttr = ordered && start > 1 ? ` start="${start}"` : ''
  const body = items.map((item) => `<li>${renderInlineMarkdown(item)}</li>`).join('')
  return `<${tag}${startAttr}>${body}</${tag}>`
}

function renderParagraph(lines: string[]): string {
  return `<p>${renderInlineMarkdown(lines.join(' '))}</p>`
}

export function renderMarkdown(value: string, options: MarkdownRenderOptions = {}): string {
  const normalized = value.replace(/\r\n/g, '\n')
  const lines = normalized.split('\n')
  const blocks: string[] = []
  let index = 0
  let headingIndex = 0

  while (index < lines.length) {
    const currentLine = lines[index]
    const trimmed = currentLine.trim()

    if (trimmed === '') {
      index += 1
      continue
    }

    const fenceMatch = isFenceLine(trimmed)
    if (fenceMatch) {
      const language = fenceMatch[1] ? ` class="language-${escapeHTML(fenceMatch[1])}"` : ''
      const codeLines: string[] = []
      index += 1
      while (index < lines.length && !isFenceLine(lines[index].trim())) {
        codeLines.push(lines[index])
        index += 1
      }
      if (index < lines.length && isFenceLine(lines[index].trim())) {
        index += 1
      }
      blocks.push(`<pre><code${language}>${escapeHTML(codeLines.join('\n'))}</code></pre>`)
      continue
    }

    const headingMatch = isHeadingLine(trimmed)
    if (headingMatch) {
      const level = headingMatch[1].length
      const text = headingMatch[2]
      let id = slugify(text)
      if (level === 2 && options.headingIds && options.headingIds[headingIndex]) {
        id = options.headingIds[headingIndex]
        headingIndex += 1
      } else if (level === 2 && options.headingIds) {
        headingIndex += 1
      }
      blocks.push(`<h${level} id="${escapeHTML(id)}">${renderInlineMarkdown(text)}</h${level}>`)
      index += 1
      continue
    }

    if (index + 1 < lines.length && isTableRow(trimmed) && isTableSeparatorLine(lines[index + 1].trim())) {
      const headerLine = currentLine
      const tableRows: string[] = []
      index += 2
      while (index < lines.length) {
        const rowLine = lines[index]
        if (!rowLine.trim()) {
          break
        }
        if (!isTableRow(rowLine)) {
          break
        }
        tableRows.push(rowLine)
        index += 1
      }
      blocks.push(renderTable(headerLine, tableRows))
      continue
    }

    const bulletMatch = isBulletLine(trimmed)
    const orderedMatch = isOrderedLine(trimmed)
    if (bulletMatch || orderedMatch) {
      const ordered = Boolean(orderedMatch)
      const items: string[] = []
      const start = orderedMatch ? Number(orderedMatch[1]) : 1
      while (index < lines.length) {
        const line = lines[index].trim()
        const match = ordered ? isOrderedLine(line) : isBulletLine(line)
        if (!match) {
          break
        }
        items.push(match[ordered ? 2 : 1])
        index += 1
      }
      blocks.push(renderList(items, ordered, start))
      continue
    }

    const paragraphLines: string[] = [trimmed]
    index += 1
    while (index < lines.length) {
      if (isBlockBoundary(lines[index])) {
        break
      }
      if (index + 1 < lines.length && isTableRow(lines[index].trim()) && isTableSeparatorLine(lines[index + 1].trim())) {
        break
      }
      paragraphLines.push(lines[index].trim())
      index += 1
    }
    blocks.push(renderParagraph(paragraphLines))
  }

  if (blocks.length === 0) {
    return ''
  }

  return blocks.join('\n')
}
