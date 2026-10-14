<script lang="ts">
  import { onMount } from 'svelte'
  import { locale, t } from '../i18n/index.svelte'
  import { loadRegistered, updateRegistered } from './client'
  import { admin } from './state.svelte'

  // The registry is only readable with the Register permission. Read it whenever the tab opens
  // (a user registered from Online would otherwise be missing).
  onMount(loadRegistered)

  const rows = $derived(
    [...(admin.registered ?? [])].sort((a, b) => a.name.localeCompare(b.name, locale())),
  )

  // The server applies writes in order, so a query right behind a write sees the result.
  function refreshSoon() {
    setTimeout(loadRegistered, 400)
  }

  function rename(id: number, name: string) {
    const next = window.prompt(t.admin.registered.renamePrompt(name), name)?.trim()
    if (!next || next === name) return
    updateRegistered([{ id, name: next }])
    refreshSoon()
  }

  function remove(id: number, name: string) {
    if (!window.confirm(t.admin.registered.removeConfirm(name))) return
    updateRegistered([{ id, name: null }])
    refreshSoon()
  }
</script>

<div class="toolbar">
  <h2 class="section">{t.admin.registered.title}</h2>
  <span class="grow"></span>
  <button type="button" onclick={loadRegistered}>{t.admin.common.refresh}</button>
</div>

{#if admin.registered === null}
  <p class="muted">{t.admin.common.loading} <span class="muted">({t.admin.registered.notLoaded})</span></p>
{:else if rows.length === 0}
  <p class="muted">{t.admin.common.empty}</p>
{:else}
  <div class="tbl-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>{t.admin.registered.id}</th>
          <th>{t.admin.common.name}</th>
          <th>{t.admin.registered.lastSeen}</th>
          <th>{t.admin.registered.lastChannel}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each rows as u (u.id)}
          <tr>
            <td>{u.id}</td>
            <td>{u.name}</td>
            <td class="muted">{u.lastSeen ?? ''}</td>
            <td class="muted">{u.lastChannel !== undefined ? (admin.channels.get(u.lastChannel)?.name ?? u.lastChannel) : ''}</td>
            <td>
              {#if u.id === 0}
                <span class="muted">{t.admin.registered.superUser}</span>
              {:else}
                <div class="actions">
                  <button type="button" class="small" onclick={() => rename(u.id, u.name)}>{t.admin.common.rename}</button>
                  <button type="button" class="small danger" onclick={() => remove(u.id, u.name)}>{t.admin.common.remove}</button>
                </div>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
