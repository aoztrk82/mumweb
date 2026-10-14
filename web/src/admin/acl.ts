// Pure helpers for the admin panel's ACL editor and ban list.
import type { AclEntry, AclGroup, BanEntry } from '../lib/protocol'

/** Mumble ACL permission bits (same values as lib/permissions.ts); `key` indexes t.admin.acl.perms. */
export const PERMS = [
  { key: 'Write', bit: 0x1 },
  { key: 'Traverse', bit: 0x2 },
  { key: 'Enter', bit: 0x4 },
  { key: 'Speak', bit: 0x8 },
  { key: 'MuteDeafen', bit: 0x10 },
  { key: 'Move', bit: 0x20 },
  { key: 'MakeChannel', bit: 0x40 },
  { key: 'LinkChannel', bit: 0x80 },
  { key: 'Whisper', bit: 0x100 },
  { key: 'TextMessage', bit: 0x200 },
  { key: 'MakeTempChannel', bit: 0x400 },
  { key: 'Listen', bit: 0x800 },
  { key: 'Kick', bit: 0x10000 },
  { key: 'Ban', bit: 0x20000 },
  { key: 'Register', bit: 0x40000 },
  { key: 'SelfRegister', bit: 0x80000 },
  { key: 'ResetUserContent', bit: 0x100000 },
] as const

/** Returns a copy of the entry with one permission bit switched in the grant or deny mask. */
export function togglePerm(e: AclEntry, field: 'grant' | 'deny', bit: number): AclEntry {
  return { ...e, [field]: (e[field] ^ bit) >>> 0 }
}

/**
 * Moves the own (non-inherited) entry at `index` one step up or down. Inherited entries
 * come first and stay put. Returns the same array when the move is not possible.
 */
export function moveEntry(list: AclEntry[], index: number, dir: -1 | 1): AclEntry[] {
  const to = index + dir
  if (index < 0 || index >= list.length || to < 0 || to >= list.length) return list
  if (list[index].inherited || list[to].inherited) return list
  const out = list.slice()
  ;[out[index], out[to]] = [out[to], out[index]]
  return out
}

export function newGroup(name: string): AclGroup {
  return { name, inherited: false, inherit: true, inheritable: true, add: [], remove: [], inheritedMembers: [] }
}

/** A ban that only matches a certificate (mumweb's own kind, address 100::/128). */
export function isCertOnly(b: BanEntry): boolean {
  return b.address === '100::' && b.mask === 128
}

/** Certificate hashes are hex SHA-1 (40 characters). */
export function validCertHash(h: string): boolean {
  return /^[0-9a-f]{40}$/i.test(h.trim())
}

export function newCertBan(hash: string, name: string, reason: string): BanEntry {
  return { address: '100::', mask: 128, hash: hash.trim().toLowerCase(), name: name.trim(), reason: reason.trim(), duration: 0 }
}

export function shortHash(h: string | undefined): string {
  return h ? h.slice(0, 8) : ''
}

/** Structural equality for plain JSON data (change detection of the editors). */
export function sameJson(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}
