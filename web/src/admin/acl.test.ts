import { describe, expect, it } from 'vitest'
import type { AclEntry } from '../lib/protocol'
import { isCertOnly, moveEntry, newCertBan, newGroup, PERMS, togglePerm, validCertHash } from './acl'

const entry = (group: string, inherited = false): AclEntry => ({ applyHere: true, applySubs: true, inherited, group, grant: 0, deny: 0 })

describe('acl helpers', () => {
  it('has unique permission bits matching Mumble', () => {
    const bits = PERMS.map((p) => p.bit)
    expect(new Set(bits).size).toBe(bits.length)
    expect(PERMS.find((p) => p.key === 'Enter')?.bit).toBe(0x4)
    expect(PERMS.find((p) => p.key === 'ResetUserContent')?.bit).toBe(0x100000)
  })

  it('toggles grant and deny independently', () => {
    let e = entry('all')
    e = togglePerm(e, 'grant', 0x4)
    e = togglePerm(e, 'deny', 0x4)
    expect(e.grant).toBe(0x4)
    expect(e.deny).toBe(0x4)
    e = togglePerm(e, 'grant', 0x4)
    expect(e.grant).toBe(0)
  })

  it('moves only own entries', () => {
    const list = [entry('inherited', true), entry('a'), entry('b')]
    expect(moveEntry(list, 1, -1)).toBe(list) // would swap with an inherited entry
    expect(moveEntry(list, 2, 1)).toBe(list) // past the end
    expect(moveEntry(list, 1, 1).map((e) => e.group)).toEqual(['inherited', 'b', 'a'])
    expect(list.map((e) => e.group)).toEqual(['inherited', 'a', 'b']) // input untouched
  })

  it('recognises certificate-only bans', () => {
    expect(isCertOnly(newCertBan('A'.repeat(40), 'x', ''))).toBe(true)
    expect(isCertOnly({ address: '1.2.3.4', mask: 128, duration: 0 })).toBe(false)
    expect(isCertOnly({ address: '100::', mask: 64, duration: 0 })).toBe(false)
  })

  it('validates certificate hashes', () => {
    expect(validCertHash('0123456789abcdef0123456789ABCDEF01234567')).toBe(true)
    expect(validCertHash('xyz')).toBe(false)
    expect(validCertHash('0123456789abcdef0123456789abcdef0123456')).toBe(false)
    expect(newCertBan(` ${'A'.repeat(40)} `, ' n ', ' r ')).toMatchObject({ hash: 'a'.repeat(40), name: 'n', reason: 'r' })
  })

  it('creates inheriting groups', () => {
    expect(newGroup('mods')).toMatchObject({ name: 'mods', inherit: true, inheritable: true, add: [], remove: [] })
  })
})
