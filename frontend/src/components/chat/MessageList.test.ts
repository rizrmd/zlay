import { describe, it, expect } from 'vitest'
import { marked } from 'marked'
import hljs from 'highlight.js'

// Configure marked similar to MessageList
marked.setOptions({
  breaks: true,
  gfm: true
})

const renderer = new marked.Renderer()

renderer.code = function(token: { text: string; lang?: string | undefined; escaped?: boolean }) {
  const code = token.text
  const language = token.lang
  
  if (language && hljs.getLanguage(language)) {
    try {
      const highlighted = hljs.highlight(code, { language: language }).value
      return `<pre class="hljs"><code class="language-${language}">${highlighted}</code></pre>`
    } catch (err) {
      console.error('Highlight.js error:', err)
    }
  }
  const highlighted = hljs.highlightAuto(code).value
  return `<pre class="hljs"><code>${highlighted}</code></pre>`
}

marked.use({ renderer })

const renderMarkdown = (content: string) => {
  try {
    return marked.parse(content)
  } catch (error) {
    console.error('Markdown parsing error:', error)
    return content
  }
}

describe('Markdown Rendering', () => {
  it('should render basic markdown', () => {
    const markdown = '# Hello World\n\nThis is **bold** and *italic* text.'
    const html = renderMarkdown(markdown)
    
    expect(html).toContain('<h1>Hello World</h1>')
    expect(html).toContain('<strong>bold</strong>')
    expect(html).toContain('<em>italic</em>')
  })

  it('should render code blocks with syntax highlighting', () => {
    const markdown = '```javascript\nconst x = 1;\n```'
    const html = renderMarkdown(markdown)
    
    expect(html).toContain('<pre class="hljs">')
    expect(html).toContain('<code class="language-javascript">')
    expect(html).toContain('<span class="hljs-keyword">const</span>')
    expect(html).toContain('<span class="hljs-number">1</span>')
  })

  it('should render inline code', () => {
    const markdown = 'This is `inline code` example.'
    const html = renderMarkdown(markdown)
    
    expect(html).toContain('<code>inline code</code>')
  })

  it('should render lists', () => {
    const markdown = '- Item 1\n- Item 2\n\n1. Numbered item\n2. Another item'
    const html = renderMarkdown(markdown)
    
    expect(html).toContain('<ul>')
    expect(html).toContain('<li>Item 1</li>')
    expect(html).toContain('<ol>')
    expect(html).toContain('<li>Numbered item</li>')
  })

  it('should render links', () => {
    const markdown = '[Google](https://google.com)'
    const html = renderMarkdown(markdown)
    
    expect(html).toContain('<a href="https://google.com">Google</a>')
  })

  it('should handle line breaks with GFM', () => {
    const markdown = 'Line 1\nLine 2\n\nLine 3'
    const html = renderMarkdown(markdown)
    
    expect(html).toContain('<p>Line 1<br>Line 2</p>')
    expect(html).toContain('<p>Line 3</p>')
  })

  it('should fallback to plain text on error', () => {
    const originalParse = marked.parse
    marked.parse = () => {
      throw new Error('Parse error')
    }
    
    const markdown = '# Test'
    const result = renderMarkdown(markdown)
    
    expect(result).toBe('# Test')
    
    // Restore original function
    marked.parse = originalParse
  })
})