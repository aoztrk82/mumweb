<script lang="ts">
  import { t } from '../i18n/index.svelte'
  import AclEditor from './AclEditor.svelte'
  import { createChannel, queryChannelPassword, removeChannel, renameChannel, setChannelPassword } from './client'
  import { admin, flatChannels } from './state.svelte'

  const rows = $derived(flatChannels())
  const userCount = (id: number) => [...admin.users.values()].filter((u) => u.channelId === id).length

  let aclChannel = $state<number | null>(null)
  let pw = $state<{ id: number; value: string; loaded: boolean } | null>(null)

  // The current channel password arrives after queryChannelPassword.
  $effect(() => {
    if (pw && !pw.loaded && admin.channelPasswords.has(pw.id)) {
      pw.value = admin.channelPasswords.get(pw.id) ?? ''
      pw.loaded = true
    }
  })

  function add(parent: number, parentName: string) {
    const name = window.prompt(t.admin.channels.createPrompt(parentName), '')?.trim()
    if (name) createChannel(parent, name)
  }

  function rename(id: number, current: string) {
    const name = window.prompt(t.admin.channels.renamePrompt, current)?.trim()
    if (name && name !== current) renameChannel(id, name)
  }

  function remove(id: number, name: string) {
    if (window.confirm(t.admin.channels.removeConfirm(name))) removeChannel(id)
  }

  function openPassword(id: number) {
    pw = { id, value: '', loaded: false }
    queryChannelPassword(id)
  }

  function submitPassword(e: SubmitEvent) {
    e.preventDefault()
    if (!pw) return
    setChannelPassword(pw.id, pw.value.trim())
    pw = null
  }
</script>

<div class="toolbar">
  <h2 class="section">{t.admin.channels.title}</h2>
</div>

<div class="tbl-wrap">
  <table class="tbl">
    <tbody>
      {#each rows as { channel: c, depth } (c.id)}
        <tr>
          <td style:padding-left="{0.7 + depth * 1.2}rem">
            <strong>{c.name}</strong>
            {#if c.temporary}<span class="tag">{t.admin.channels.temporary}</span>{/if}
            {#if c.restricted}<span class="tag">{t.admin.channels.restricted}</span>{/if}
            <span class="muted">· {t.admin.channels.users(userCount(c.id))}</span>
          </td>
          <td>
            <div class="actions">
              <button type="button" class="small" onclick={() => add(c.id, c.name)}>{t.admin.channels.create}</button>
              {#if c.id !== 0}
                <button type="button" class="small" onclick={() => rename(c.id, c.name)}>{t.admin.common.rename}</button>
              {/if}
              <button type="button" class="small" onclick={() => openPassword(c.id)}>{t.admin.channels.password}</button>
              <button type="button" class="small" onclick={() => (aclChannel = c.id)}>{t.admin.channels.acl}</button>
              {#if c.id !== 0}
                <button type="button" class="small danger" onclick={() => remove(c.id, c.name)}>{t.admin.common.remove}</button>
              {/if}
            </div>
          </td>
        </tr>
        {#if pw?.id === c.id}
          <tr>
            <td colspan="2">
              <form class="pw" onsubmit={submitPassword}>
                <label for="pw-{c.id}">{t.admin.channels.passwordPrompt(c.name)}</label>
                <div class="row">
                  <input id="pw-{c.id}" type="text" bind:value={pw.value} disabled={!pw.loaded} autocomplete="off" maxlength="128" />
                  <button type="submit" class="primary" disabled={!pw.loaded}>{t.admin.common.save}</button>
                  <button type="button" onclick={() => (pw = null)}>{t.admin.common.cancel}</button>
                </div>
              </form>
            </td>
          </tr>
        {/if}
        {#if aclChannel === c.id}
          <tr>
            <td colspan="2"><AclEditor channelId={c.id} channelName={c.name} onclose={() => (aclChannel = null)} /></td>
          </tr>
        {/if}
      {/each}
    </tbody>
  </table>
</div>

<style>
  .pw label {
    display: block;
    margin-bottom: 0.3rem;
    font-size: 0.9rem;
  }
  .row {
    display: flex;
    gap: 0.5rem;
  }
</style>
