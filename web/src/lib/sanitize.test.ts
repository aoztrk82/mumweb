// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import { sanitizeHtml, textToHtml } from './sanitize'

// Mumble chat, comments and descriptions are attacker-controlled HTML that ends up in {@html}.

const PNG = 'data:image/png;base64,iVBORw0KGgo='

/** Parses sanitized output the way the page would and reports anything executable. */
function dangers(html: string): string[] {
  const doc = new DOMParser().parseFromString(`<body>${sanitizeHtml(html)}</body>`, 'text/html')
  const found: string[] = []
  for (const el of doc.body.querySelectorAll('*')) {
    if (['SCRIPT', 'IFRAME', 'OBJECT', 'EMBED', 'STYLE', 'LINK', 'META', 'FORM', 'INPUT', 'SVG', 'MATH', 'BASE'].includes(el.tagName.toUpperCase())) found.push('tag ' + el.tagName)
    for (const a of el.getAttributeNames()) {
      if (a.startsWith('on')) found.push('attr ' + a)
      if (a === 'style') found.push('style attr')
      if (['href', 'src'].includes(a) && /^\s*(javascript|vbscript|data:text|data:application)/i.test(el.getAttribute(a) ?? '')) found.push(a + '=' + el.getAttribute(a))
    }
  }
  return found
}

describe('sanitizeHtml', () => {
  const payloads = [
    '<script>alert(1)</script>',
    '<img src=x onerror=alert(1)>',
    '<svg onload=alert(1)>',
    '<svg><script>alert(1)</script></svg>',
    '<a href="javascript:alert(1)">x</a>',
    '<a href="JaVaScRiPt:alert(1)">x</a>',
    '<a href="&#106;avascript:alert(1)">x</a>',
    '<a href="data:text/html,<script>alert(1)</script>">x</a>',
    '<iframe src="https://evil.example"></iframe>',
    '<iframe srcdoc="<script>alert(1)</script>"></iframe>',
    '<object data="x"></object><embed src="x">',
    '<form action="https://evil.example"><input name=p></form>',
    '<style>body{background:url(https://evil.example)}</style>',
    '<p style="background:url(https://evil.example)">x</p>',
    '<meta http-equiv="refresh" content="0;url=https://evil.example">',
    '<base href="https://evil.example/">',
    '<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>',
    '<b onmouseover=alert(1)>x</b>',
    '<img src="data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==">',
    '<img src="https://evil.example/track.png">',
    '<<script>script>alert(1)<</script>/script>',
  ]
  for (const p of payloads) {
    it(`neutralises ${p.slice(0, 50)}`, () => {
      expect(dangers(p)).toEqual([])
    })
  }

  it('does not load remote or non-image sources', () => {
    expect(sanitizeHtml('<img src="https://evil.example/x.png">')).not.toContain('<img')
    expect(sanitizeHtml('<img src="data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=">')).not.toContain('<img')
    expect(sanitizeHtml(`<img src="${PNG}" alt="pic">`)).toContain('<img')
  })

  it('keeps ordinary formatting and safe links, and hardens the links', () => {
    const out = sanitizeHtml('<b>fett</b> <i>x</i> <a href="https://example.org/a?b=1">l</a> <a href="mailto:a@b.c">m</a>')
    expect(out).toContain('<b>fett</b>')
    expect(out).toMatch(/<a href="https:\/\/example\.org\/a\?b=1" target="_blank" rel="noopener noreferrer nofollow">l<\/a>/)
    expect(out).toContain('mailto:a@b.c')
  })

  it('drops the href of links with other schemes', () => {
    expect(sanitizeHtml('<a href="ftp://x">f</a><a href="//evil.example">p</a><a href="/relative">r</a>')).not.toContain('href')
  })
})

describe('textToHtml', () => {
  it('escapes typed text so it cannot become markup on the other side', () => {
    const html = textToHtml('<img src=x onerror=alert(1)> & "q"\nline2')
    expect(html).not.toContain('<img')
    expect(html).toBe('&lt;img src=x onerror=alert(1)&gt; &amp; &quot;q&quot;<br>line2')
    expect(dangers(html)).toEqual([])
  })
})
