<script lang="ts">
  import { locale, t } from '../i18n/index.svelte'
  import { createInvite } from './client'
  import { admin } from './state.svelte'

  const ttlOptions = $derived([
    { value: 3600, label: t.admin.invites.ttl1h },
    { value: 6 * 3600, label: t.admin.invites.ttl6h },
    { value: 86400, label: t.admin.invites.ttl1d },
    { value: 3 * 86400, label: t.admin.invites.ttl3d },
    { value: 7 * 86400, label: t.admin.invites.ttl7d },
    { value: 30 * 86400, label: t.admin.invites.ttl30d },
  ])
  let ttl = $state(86400)
  let copiedToken = $state<string | null>(null)

  function create(e: SubmitEvent) {
    e.preventDefault()
    createInvite(ttl)
  }

  async function copy(url: string, token: string) {
    try {
      await navigator.clipboard.writeText(url)
      copiedToken = token
      setTimeout(() => {
        if (copiedToken === token) copiedToken = null
      }, 1500)
    } catch {
      // Clipboard API unavailable or denied; nothing more we can do.
    }
  }

  function remove(token: string) {
    admin.invites = admin.invites.filter((i) => i.token !== token)
  }

  function formatExpiry(unixSeconds: number): string {
    return new Date(unixSeconds * 1000).toLocaleString(locale())
  }
</script>

<div class="toolbar">
  <h2 class="section">{t.admin.invites.title}</h2>
</div>
<p class="muted note">{t.admin.invites.hint}</p>

<form class="create" onsubmit={create}>
  <label>
    <span>{t.admin.invites.ttl}</span>
    <select bind:value={ttl}>
      {#each ttlOptions as opt (opt.value)}
        <option value={opt.value}>{opt.label}</option>
      {/each}
    </select>
  </label>
  <button type="submit" class="primary">{t.admin.invites.create}</button>
</form>

{#if admin.invites.length === 0}
  <p class="muted">{t.admin.invites.empty}</p>
{:else}
  <div class="tbl-wrap">
    <table class="tbl">
      <tbody>
        {#each admin.invites as inv (inv.token)}
          <tr>
            <td><code class="link">{inv.url}</code></td>
            <td class="muted">{t.admin.invites.expires(formatExpiry(inv.expiresAt))}</td>
            <td class="actions">
              <button type="button" class="small" onclick={() => copy(inv.url, inv.token)}>
                {copiedToken === inv.token ? t.admin.invites.copied : t.admin.invites.copy}
              </button>
              <button type="button" class="small danger" onclick={() => remove(inv.token)}>{t.admin.invites.remove}</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .note {
    margin: 0 0 0.75rem;
    font-size: 0.85rem;
  }
  .create {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 0.75rem;
    margin-bottom: 1rem;
  }
  .create label {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    font-size: 0.85rem;
  }
  .link {
    font-size: 0.85rem;
    word-break: break-all;
  }
</style>
