// HTML from Mumble (chat, welcome text, descriptions) is untrusted.
import DOMPurify, { type DOMPurify as Purifier } from 'dompurify'

const ALLOWED_TAGS = ['b', 'i', 'u', 's', 'strong', 'em', 'a', 'br', 'p', 'img', 'span']
const ALLOWED_ATTR = ['href', 'src', 'alt', 'title']

let purify: Purifier | null = null

// Created on first use: DOMPurify needs a DOM, which tests running in Node lack.
function instance(): Purifier {
  if (purify) return purify
  purify = DOMPurify()
  purify.addHook('afterSanitizeAttributes', restrictLinksAndImages)
  return purify
}

function restrictLinksAndImages(node: Element): void {
  if (node.tagName === 'A') {
    const href = node.getAttribute('href') ?? ''
    if (!/^(https?:|mailto:)/i.test(href)) {
      node.removeAttribute('href')
    } else {
      node.setAttribute('target', '_blank')
      node.setAttribute('rel', 'noopener noreferrer nofollow')
    }
  }
  // Only inline data: images (how Mumble sends pasted images); no remote loads.
  if (node.tagName === 'IMG') {
    const src = node.getAttribute('src') ?? ''
    if (!/^data:image\/(png|jpeg|gif|webp);base64,/i.test(src)) {
      node.remove()
    }
  }
}

export function sanitizeHtml(html: string): string {
  return instance().sanitize(html, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    ADD_DATA_URI_TAGS: ['img'],
    ALLOW_DATA_ATTR: false,
  })
}

/** Escapes plain text typed by the user for sending as Mumble HTML. */
export function textToHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/\n/g, '<br>')
}
