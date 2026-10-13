// Export/import of the browser's Mumble identity as a PEM file.
import type { CertPair } from './protocol'

const CERT_RE = /-----BEGIN CERTIFICATE-----[\s\S]+?-----END CERTIFICATE-----/
const KEY_RE = /-----BEGIN (?:RSA |EC )?PRIVATE KEY-----[\s\S]+?-----END (?:RSA |EC )?PRIVATE KEY-----/

export function certToPem(c: CertPair): string {
  return `${c.pem.trim()}\n${c.key.trim()}\n`
}

/** Parses a PEM bundle with one certificate and its private key. Returns null if either is missing. */
export function parsePemBundle(text: string): CertPair | null {
  const pem = text.match(CERT_RE)?.[0]
  const key = text.match(KEY_RE)?.[0]
  if (!pem || !key) return null
  return { pem: pem + '\n', key: key + '\n' }
}

export function downloadText(filename: string, text: string): void {
  const url = URL.createObjectURL(new Blob([text], { type: 'application/x-pem-file' }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
