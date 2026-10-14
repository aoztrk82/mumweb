<script lang="ts">
  import { onMount } from 'svelte'
  import { t } from '../i18n/index.svelte'
  import type { AclData } from '../lib/protocol'
  import { moveEntry, newGroup, PERMS, sameJson, togglePerm } from './acl'
  import { loadAcl, loadRegistered, saveAcl } from './client'
  import { admin } from './state.svelte'

  let { channelId, channelName, onclose }: { channelId: number; channelName: string; onclose: () => void } = $props()

  // Editable copy of the channel's ACL; null until the server's answer arrives.
  let draft = $state<AclData | null>(null)
  let original = $state<AclData | null>(null)
  let newKind = $state<'group' | 'user'>('group')
  let newGroupName = $state('')
  let newUser = $state('')
  let newGroupForGroups = $state('')

  // One component per open channel (ChannelsTab recreates it); a plain onMount keeps the
  // arriving user list from re-triggering the query and resetting the draft.
  onMount(() => {
    admin.acl = null
    loadAcl(channelId)
    if (admin.registered === null) loadRegistered()
  })

  $effect(() => {
    const a = admin.acl
    if (a && a.channelId === channelId) {
      const { channelId: _id, ...data } = $state.snapshot(a)
      original = data
      draft = structuredClone(data)
    }
  })

  const dirty = $derived(draft !== null && original !== null && !sameJson(draft, original))
  const users = $derived(admin.registered ?? [])
  const name = (id: number) => admin.userName(id, t.admin.common.unknownUser)

  function who(e: AclData['acls'][number]): string {
    return e.userId !== undefined ? name(e.userId) : (e.group ?? '')
  }

  function addEntry() {
    if (!draft) return
    if (newKind === 'group') {
      const g = newGroupName.trim()
      if (!g) return
      draft.acls.push({ applyHere: true, applySubs: true, inherited: false, group: g, grant: 0, deny: 0 })
      newGroupName = ''
    } else {
      if (newUser === '') return
      draft.acls.push({ applyHere: true, applySubs: true, inherited: false, userId: Number(newUser), grant: 0, deny: 0 })
      newUser = ''
    }
  }

  function addGroup() {
    const g = newGroupForGroups.trim()
    if (!draft || !g || draft.groups.some((x) => x.name === g)) return
    draft.groups.push(newGroup(g))
    newGroupForGroups = ''
  }

  function addMember(list: number[], value: string) {
    const id = Number(value)
    if (value !== '' && !list.includes(id)) list.push(id)
  }

  function save() {
    if (!draft) return
    saveAcl(channelId, $state.snapshot(draft))
    admin.toast('info', t.admin.acl.saved)
    // Re-read instead of trusting the draft: the server may have refused part of it.
    original = null
    draft = null
    admin.acl = null
    setTimeout(() => loadAcl(channelId), 400)
  }
</script>

<section class="acl">
  <div class="toolbar">
    <h3>{t.admin.acl.title(channelName)}</h3>
    <span class="grow"></span>
    {#if dirty}<span class="muted">{t.admin.common.unsaved}</span>{/if}
    <button type="button" class="primary" disabled={!dirty} onclick={save}>{t.admin.common.save}</button>
    <button type="button" onclick={onclose}>{t.admin.acl.close}</button>
  </div>

  {#if draft === null}
    <p class="muted">{t.admin.common.loading}</p>
  {:else}
    <label class="check">
      <input type="checkbox" bind:checked={draft.inheritAcls} />
      <span>{t.admin.acl.inheritAcls}</span>
    </label>

    <h4>{t.admin.acl.entries}</h4>
    <p class="muted small">{t.admin.acl.entriesNote}</p>
    {#each draft.acls as e, i (i)}
      <details class="entry" class:inherited={e.inherited}>
        <summary>
          <strong>{who(e)}</strong>
          <span class="tag">{e.userId !== undefined ? t.admin.acl.user : t.admin.acl.group}</span>
          {#if e.inherited}<span class="tag">{t.admin.acl.inheritedTag}</span>{/if}
          <span class="muted small">
            {PERMS.filter((p) => e.grant & p.bit).map((p) => '+' + p.key).join(' ')}
            {PERMS.filter((p) => e.deny & p.bit).map((p) => '−' + p.key).join(' ')}
          </span>
        </summary>
        <fieldset disabled={e.inherited}>
          <div class="flags">
            <label class="check"><input type="checkbox" bind:checked={e.applyHere} /><span>{t.admin.acl.here}</span></label>
            <label class="check"><input type="checkbox" bind:checked={e.applySubs} /><span>{t.admin.acl.subs}</span></label>
          </div>
          <table class="perms">
            <thead>
              <tr><th>{t.admin.acl.permissions}</th><th>{t.admin.acl.grant}</th><th>{t.admin.acl.deny}</th></tr>
            </thead>
            <tbody>
              {#each PERMS as p (p.key)}
                <tr>
                  <td>{t.admin.acl.perms[p.key]}</td>
                  <td><input type="checkbox" checked={(e.grant & p.bit) !== 0} onchange={() => (draft!.acls[i] = togglePerm(e, 'grant', p.bit))} aria-label="{t.admin.acl.grant}: {t.admin.acl.perms[p.key]}" /></td>
                  <td><input type="checkbox" checked={(e.deny & p.bit) !== 0} onchange={() => (draft!.acls[i] = togglePerm(e, 'deny', p.bit))} aria-label="{t.admin.acl.deny}: {t.admin.acl.perms[p.key]}" /></td>
                </tr>
              {/each}
            </tbody>
          </table>
          <div class="actions">
            <button type="button" class="small" onclick={() => (draft!.acls = moveEntry(draft!.acls, i, -1))}>↑ {t.admin.acl.moveUp}</button>
            <button type="button" class="small" onclick={() => (draft!.acls = moveEntry(draft!.acls, i, 1))}>↓ {t.admin.acl.moveDown}</button>
            <button type="button" class="small danger" onclick={() => draft!.acls.splice(i, 1)}>{t.admin.common.remove}</button>
          </div>
        </fieldset>
      </details>
    {/each}

    <div class="add">
      <select bind:value={newKind} aria-label={t.admin.acl.who}>
        <option value="group">{t.admin.acl.group}</option>
        <option value="user">{t.admin.acl.user}</option>
      </select>
      {#if newKind === 'group'}
        <input type="text" bind:value={newGroupName} placeholder={t.admin.acl.groupName} maxlength="128" />
      {:else}
        <select bind:value={newUser} aria-label={t.admin.acl.user}>
          <option value="">{t.admin.acl.userPick}</option>
          {#each users as u (u.id)}<option value={String(u.id)}>{u.name}</option>{/each}
        </select>
      {/if}
      <button type="button" onclick={addEntry}>{t.admin.acl.addEntry}</button>
    </div>

    <h4>{t.admin.acl.groupsTitle}</h4>
    <p class="muted small">{t.admin.acl.groupsNote}</p>
    {#each draft.groups as g, gi (g.name)}
      <details class="entry" class:inherited={g.inherited}>
        <summary>
          <strong>{g.name}</strong>
          {#if g.inherited}<span class="tag">{t.admin.acl.inheritedTag}</span>{/if}
          <span class="muted small">{g.add.length} / {g.remove.length}</span>
        </summary>
        <div class="flags">
          <label class="check"><input type="checkbox" bind:checked={g.inherit} /><span>{t.admin.acl.inherit}</span></label>
          <label class="check"><input type="checkbox" bind:checked={g.inheritable} /><span>{t.admin.acl.inheritable}</span></label>
        </div>

        {#each [{ key: 'add', label: t.admin.acl.members }, { key: 'remove', label: t.admin.acl.excluded }] as const as list (list.key)}
          <div class="members">
            <span class="muted small">{list.label}</span>
            {#each g[list.key] as id (id)}
              <span class="chip">{name(id)} <button type="button" aria-label={t.admin.common.remove} onclick={() => (g[list.key] = g[list.key].filter((x) => x !== id))}>×</button></span>
            {/each}
            <select
              aria-label="{list.label}: {t.admin.acl.addMember}"
              onchange={(ev) => {
                addMember(g[list.key], ev.currentTarget.value)
                ev.currentTarget.value = ''
              }}
            >
              <option value="">{t.admin.acl.addMember}</option>
              {#each users.filter((u) => !g[list.key].includes(u.id)) as u (u.id)}<option value={String(u.id)}>{u.name}</option>{/each}
            </select>
          </div>
        {/each}
        {#if g.inheritedMembers.length > 0}
          <div class="members">
            <span class="muted small">{t.admin.acl.inheritedMembers}</span>
            {#each g.inheritedMembers as id (id)}<span class="chip dim">{name(id)}</span>{/each}
          </div>
        {/if}
        {#if !g.inherited}
          <div class="actions"><button type="button" class="small danger" onclick={() => draft!.groups.splice(gi, 1)}>{t.admin.common.remove}</button></div>
        {/if}
      </details>
    {/each}

    <div class="add">
      <input type="text" bind:value={newGroupForGroups} placeholder={t.admin.acl.addGroup} maxlength="128" />
      <button type="button" onclick={addGroup}>{t.admin.acl.addGroup}</button>
    </div>
  {/if}
</section>

<style>
  .acl {
    padding: 0.6rem 0.3rem 0.9rem;
  }
  h3 {
    margin: 0;
    font-size: 1rem;
  }
  h4 {
    margin: 1rem 0 0.2rem;
    font-size: 0.95rem;
  }
  .small {
    font-size: 0.8rem;
    margin: 0 0 0.4rem;
  }
  .check {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }
  .entry {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 0.4rem 0.6rem;
    margin-bottom: 0.4rem;
  }
  .entry.inherited {
    opacity: 0.75;
  }
  summary {
    cursor: pointer;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
  }
  fieldset {
    border: 0;
    padding: 0;
    margin: 0.5rem 0 0;
    min-width: 0;
  }
  .flags {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    margin: 0.5rem 0;
  }
  .perms {
    border-collapse: collapse;
    font-size: 0.88rem;
    margin-bottom: 0.5rem;
  }
  .perms th,
  .perms td {
    padding: 0.15rem 0.7rem 0.15rem 0;
    text-align: left;
  }
  .add {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin: 0.5rem 0;
  }
  .add input[type='text'] {
    flex: 1 1 220px;
    width: auto;
  }
  .members {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.35rem;
    margin: 0.4rem 0;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.1rem 0.2rem 0.1rem 0.55rem;
    border: 1px solid var(--border);
    border-radius: 999px;
    font-size: 0.85rem;
  }
  .chip.dim {
    padding-right: 0.55rem;
    color: var(--text-dim);
  }
  .chip button {
    border: 0;
    background: transparent;
    padding: 0 0.3rem;
    line-height: 1;
  }
</style>
