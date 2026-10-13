// Mumble ACL permission bits (src/ACL.h) and the checks the desktop client uses
// to enable context-menu entries (MainWindow::qmUser_aboutToShow / updateMenuPermissions).
import { session } from './stores/session.svelte'

export const Perm = {
  Write: 0x1,
  Traverse: 0x2,
  Enter: 0x4,
  Speak: 0x8,
  MuteDeafen: 0x10,
  Move: 0x20,
  MakeChannel: 0x40,
  LinkChannel: 0x80,
  Whisper: 0x100,
  TextMessage: 0x200,
  MakeTempChannel: 0x400,
  Listen: 0x800,
  Kick: 0x10000,
  Ban: 0x20000,
  Register: 0x40000,
  SelfRegister: 0x80000,
  ResetUserContent: 0x100000,
  All: 0x1f0fff,
} as const

/**
 * SuperUser has every permission except Speak and Whisper (Mumble's ChanACL::effectivePermissions);
 * the server never sends it permission updates.
 */
const SUPERUSER_PERMS = 0x1f0fff & ~(0x8 | 0x100)

export function isSuperUser(): boolean {
  return session.self?.userId === 0
}

/** Root-channel permissions (kick, ban, register, reset content). */
export function rootPerms(): number {
  return isSuperUser() ? SUPERUSER_PERMS : session.rootPermissions
}

/**
 * Permissions in a channel. Unknown channels are queried and, like the desktop
 * client, optimistically treated as allowed (root: the ServerSync value) until
 * the answer arrives; the server still enforces everything.
 */
export function channelPerms(channelId: number, query: (id: number) => void): number {
  if (isSuperUser()) return SUPERUSER_PERMS
  const known = session.channelPermissions.get(channelId)
  if (known !== undefined) return known
  query(channelId)
  return channelId === 0 ? session.rootPermissions : Perm.All
}

export const has = (perms: number, ...bits: number[]) => bits.some((b) => (perms & b) !== 0)
