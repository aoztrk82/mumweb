<script lang="ts">
  import { onMount } from 'svelte'
  import { t } from '../i18n/index.svelte'
  import type { BanEntry } from '../lib/protocol'
  import { isCertOnly, newCertBan, sameJson, shortHash, validCertHash } from './acl'
  import { loadBans, saveBans } from './client'
  import { admin } from './state.svelte'

  // Edited copy of the server's list; null until the first answer arrives.
  let draft = $state<BanEntry[] | null>(null)
  let original = $state<BanEntry[] | null>(null)
  let newHash = $state('')
  let newName = $state('')
  let newReason = $state('')
  let hashError = $state(false)

  // Always read the list when the tab opens: it may have changed elsewhere (a ban from Online,
  // the desktop client) and a stale draft saved back would undo those changes.
  onMount(loadBans)

  // A fresh list from the server replaces the draft.
  $effect(() => {
    if (admin.bans) {
      original = $state.snapshot(admin.bans)
      draft = $state.snapshot(admin.bans)
    }
  })

  const dirty = $derived(draft !== null && original !== null && !sameJson(draft, original))
  const removedCount = $derived.by(() => {
    const kept = draft
    return kept && original ? original.filter((o) => !kept.some((d) => d.address === o.address && d.mask === o.mask && d.hash === o.hash)).length : 0
  })

  function remove(i: number) {
    draft = draft!.filter((_, j) => j !== i)
  }

  function addBan(e: SubmitEvent) {
    e.preventDefault()
    if (!validCertHash(newHash)) {
      hashError = true
      return
    }
    hashError = false
    draft = [...draft!, newCertBan(newHash, newName, newReason)]
    newHash = newName = newReason = ''
  }

  function save() {
    saveBans($state.snapshot(draft!))
    // Re-read the list instead of trusting the draft: the server may have refused it.
    loadBans()
  }

  function duration(b: BanEntry): string {
    return b.duration ? t.admin.bans.seconds(b.duration) : t.admin.bans.permanent
  }
</script>

<div class="toolbar">
  <h2 class="section">{t.admin.bans.title}</h2>
  <span class="grow"></span>
  {#if dirty}<span class="muted">{t.admin.common.unsaved} {removedCount > 0 ? t.admin.bans.removed(removedCount) : ''}</span>{/if}
  <button type="button" onclick={loadBans}>{t.admin.common.refresh}</button>
  <button type="button" class="primary" disabled={!dirty} onclick={save}>{t.admin.common.save}</button>
</div>
<p class="muted note">{t.admin.bans.note}</p>

{#if draft === null}
  <p class="muted">{t.admin.common.loading} <span class="muted">({t.admin.bans.notLoaded})</span></p>
{:else}
  {#if draft.length === 0}
    <p class="muted">{t.admin.common.empty}</p>
  {:else}
    <div class="tbl-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th>{t.admin.bans.name}</th>
            <th>{t.admin.bans.target}</th>
            <th>{t.admin.bans.reason}</th>
            <th>{t.admin.bans.start}</th>
            <th>{t.admin.bans.duration}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each draft as b, i (b.address + '/' + b.mask + '/' + (b.hash ?? '') + '/' + i)}
            <tr>
              <td>{b.name ?? ''}</td>
              <td>
                {#if isCertOnly(b)}
                  <span class="tag">{t.admin.bans.certOnly}</span> <code title={b.hash}>{shortHash(b.hash)}</code>
                {:else}
                  <code>{b.address}/{b.mask}</code>
                  {#if b.hash}<code title={b.hash}>{shortHash(b.hash)}</code>{/if}
                {/if}
              </td>
              <td>{b.reason ?? ''}</td>
              <td class="muted">{b.start ?? ''}</td>
              <td class="muted">{duration(b)}</td>
              <td><button type="button" class="small danger" onclick={() => remove(i)}>{t.admin.common.remove}</button></td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <form class="add" onsubmit={addBan}>
    <h3>{t.admin.bans.addTitle}</h3>
    <div class="fields">
      <input type="text" bind:value={newHash} placeholder={t.admin.bans.hashPlaceholder} aria-label={t.admin.bans.hash} spellcheck="false" />
      <input type="text" bind:value={newName} placeholder={t.admin.bans.name} aria-label={t.admin.bans.name} maxlength="128" />
      <input type="text" bind:value={newReason} placeholder={t.admin.bans.reason} aria-label={t.admin.bans.reason} maxlength="256" />
      <button type="submit">{t.admin.common.add}</button>
    </div>
    {#if hashError}<p class="error" role="alert">{t.admin.bans.invalidHash}</p>{/if}
  </form>
{/if}

<style>
  .note {
    margin: 0 0 0.75rem;
    font-size: 0.85rem;
  }
  code {
    font-size: 0.85rem;
  }
  .add {
    margin-top: 1.25rem;
  }
  h3 {
    margin: 0 0 0.5rem;
    font-size: 1rem;
  }
  .fields {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .fields input {
    flex: 1 1 200px;
    width: auto;
  }
  .error {
    margin: 0.4rem 0 0;
    font-size: 0.9rem;
    color: var(--danger);
  }
</style>
