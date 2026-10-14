<script lang="ts">
  import { locale, t } from '../i18n/index.svelte'
  import { banUser, kickUser, registerUser, setUser } from './client'
  import { admin } from './state.svelte'

  const rows = $derived(
    // This tab's own account first (it sits in the root channel), the rest alphabetical.
    [...admin.users.values()].sort(
      (a, b) =>
        Number(b.session === admin.selfSession) - Number(a.session === admin.selfSession) ||
        a.name.localeCompare(b.name, locale()),
    ),
  )

  function kick(session: number, name: string) {
    const reason = window.prompt(t.admin.online.kickReason(name), '')
    if (reason !== null) kickUser(session, reason)
  }

  function ban(session: number, name: string) {
    const reason = window.prompt(t.admin.online.banReason(name), '')
    if (reason !== null) banUser(session, reason)
  }

  function states(u: (typeof rows)[number]): string[] {
    const s: string[] = []
    if (u.mute) s.push(t.admin.online.muted)
    if (u.deaf) s.push(t.admin.online.deafened)
    if (u.selfMute) s.push(t.admin.online.selfMuted)
    if (u.selfDeaf) s.push(t.admin.online.selfDeafened)
    return s
  }
</script>

<div class="tbl-wrap">
  <table class="tbl">
    <thead>
      <tr>
        <th>{t.admin.online.user}</th>
        <th>{t.admin.online.channel}</th>
        <th>{t.admin.online.state}</th>
        <th>{t.admin.online.actions}</th>
      </tr>
    </thead>
    <tbody>
      {#each rows as u (u.session)}
        {@const self = u.session === admin.selfSession}
        <tr>
          <td>
            {u.name}
            <span class="tag">{u.registered ? t.admin.online.registered : t.admin.online.guest}</span>
            {#if self}<span class="tag">{t.admin.online.you}</span>{/if}
          </td>
          <td>{admin.channels.get(u.channelId)?.name ?? u.channelId}</td>
          <td class="muted">{states(u).join(', ')}</td>
          <td>
            {#if !self}
              <div class="actions">
                <button type="button" class="small danger" onclick={() => kick(u.session, u.name)}>{t.admin.online.kick}</button>
                <button type="button" class="small danger" disabled={!u.hash} onclick={() => ban(u.session, u.name)}>{t.admin.online.ban}</button>
                <button type="button" class="small" onclick={() => setUser(u.session, { mute: !u.mute })}>
                  {u.mute ? t.admin.online.unmute : t.admin.online.mute}
                </button>
                <button type="button" class="small" onclick={() => setUser(u.session, { deaf: !u.deaf })}>
                  {u.deaf ? t.admin.online.undeafen : t.admin.online.deafen}
                </button>
                {#if !u.registered}
                  <button type="button" class="small" disabled={!u.hash} onclick={() => registerUser(u.session)}>{t.admin.online.register}</button>
                {/if}
              </div>
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
