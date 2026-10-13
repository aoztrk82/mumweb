import { describe, expect, it } from 'vitest'
import { decodeAudioFrame, encodeAudioFrame } from './protocol'
import { textToHtml } from './sanitize'
import { backoffSeconds } from './ws'

describe('audio frames', () => {
  it('encodes browser -> gateway frames', () => {
    const buf = encodeAudioFrame(258, new Uint8Array([0xf8, 0xff, 0xfe]), true)
    expect([...new Uint8Array(buf)]).toEqual([0x01, 0x01, 2, 1, 0, 0, 0, 0, 0, 0, 0xf8, 0xff, 0xfe])
  })

  it('decodes gateway -> browser frames (same bytes as the Go test)', () => {
    const bytes = new Uint8Array([0x01, 0x01, 7, 0, 0, 0, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0x80, 0x3f, 9, 8])
    const p = decodeAudioFrame(bytes.buffer)
    expect(p).not.toBeNull()
    expect(p!.sender).toBe(7)
    expect(p!.frameNumber).toBe(258)
    expect(p!.volumeAdjustment).toBe(1)
    expect(p!.terminator).toBe(true)
    expect([...p!.opus]).toEqual([9, 8])
  })

  it('rejects malformed frames', () => {
    expect(decodeAudioFrame(new ArrayBuffer(5))).toBeNull()
    const wrongType = new Uint8Array(20)
    wrongType[0] = 0x02
    expect(decodeAudioFrame(wrongType.buffer)).toBeNull()
  })
})

describe('reconnect backoff', () => {
  it('doubles up to 30 s', () => {
    expect([1, 2, 3, 4, 5, 6, 7, 20].map(backoffSeconds)).toEqual([1, 2, 4, 8, 16, 30, 30, 30])
  })
})

describe('textToHtml', () => {
  it('escapes markup and keeps line breaks', () => {
    expect(textToHtml('<b>a</b> & "b"\nc')).toBe('&lt;b&gt;a&lt;/b&gt; &amp; &quot;b&quot;<br>c')
  })
})
