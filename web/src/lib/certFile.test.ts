import { describe, expect, it } from 'vitest'
import { certToPem, parsePemBundle } from './certFile'

const cert = '-----BEGIN CERTIFICATE-----\nAAA\n-----END CERTIFICATE-----\n'
const key = '-----BEGIN PRIVATE KEY-----\nBBB\n-----END PRIVATE KEY-----\n'

describe('PEM bundle', () => {
  it('round-trips', () => {
    expect(parsePemBundle(certToPem({ pem: cert, key }))).toEqual({ pem: cert, key })
  })
  it('accepts RSA keys and surrounding text, in any order', () => {
    const rsa = key.replace(/PRIVATE KEY/g, 'RSA PRIVATE KEY')
    expect(parsePemBundle(`comment\n${rsa}\n${cert}`)?.key).toContain('RSA PRIVATE KEY')
  })
  it('rejects incomplete bundles', () => {
    expect(parsePemBundle(cert)).toBeNull()
    expect(parsePemBundle(key)).toBeNull()
  })
})
