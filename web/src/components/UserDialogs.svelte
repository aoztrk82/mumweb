<script lang="ts">
  import { tr } from '../i18n/tr'
  import { userCommand } from '../lib/client'
  import { sanitizeHtml } from '../lib/sanitize'
  import { session } from '../lib/stores/session.svelte'
  import { ui } from '../lib/stores/ui.svelte'
  import { displayName } from '../lib/stores/userPrefs.svelte'

  let dialog: HTMLDialogElement | undefined = $state()
  let value = $state('')

  const d = $derived(ui.dialog)
  const target = $derived(d && d.kind !== 'prompt' ? session.users.get(d.session) : undefined)
  const stats = $derived(d?.kind === 'info' && session.userStats?.session === d.session ? session.userStats : null)

  $effect(() => {
    if (!dialog) return
    if (d && !dialog.open) {
      if (d.kind === 'prompt') value = d.value
      dialog.showModal()
    } else if (!d && dialog.open) {
      dialog.close()
    }
  })

  function close() {
    ui.dialog = null
  }

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (d?.kind !== 'prompt') return
    if (!d.allowEmpty && !value.trim()) return
    d.onSubmit(value)
    close()
  }

  function onKey(e: KeyboardEvent) {
    // Ctrl/Cmd+Enter submits a multi-line message.
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) (e.currentTarget as HTMLTextAreaElement).form?.requestSubmit()
  }

  function duration(secs?: number): string {
    if (secs === undefined) return '—'
    const h = Math.floor(secs / 3600)
    const m = Math.floor((secs % 3600) / 60)
    const s = secs % 60
    return h ? `${h} sa ${m} dk` : m ? `${m} dk ${s} sn` : `${s} sn`
  }

  const packets = (p?: { good: number; late: number; lost: number }) => (p ? `${p.good} / ${p.late} / ${p.lost}` : '—')
</script>

<dialog bind:this={dialog} onclose={close} aria-labelledby="user-dialog-title">
  {#if d?.kind === 'prompt'}
    <form onsubmit={submit}>
      <h2 id="user-dialog-title">{d.title}</h2>
      <label class="field">
        <span>{d.label}</span>
        {#if d.multiline}
          <!-- svelte-ignore a11y_autofocus -->
          <textarea rows="4" bind:value onkeydown={onKey} autofocus></textarea>
        {:else}
          <!-- svelte-ignore a11y_autofocus -->
          <input type="text" bind:value maxlength="200" autofocus />
        {/if}
      </label>
      {#if d.hint}<p class="hint">{d.hint}</p>{/if}
      <div class="actions">
        <button type="button" onclick={close}>{tr.channels.cancel}</button>
        <button type="submit" class:primary={!d.danger} class:danger={d.danger} disabled={!d.allowEmpty && !value.trim()}>
          {d.confirm}
        </button>
      </div>
    </form>
  {:else if d?.kind === 'comment'}
    <h2 id="user-dialog-title">{tr.userMenu.commentTitle(target ? displayName(target) : '')}</h2>
    <div class="comment">
      {#if target?.comment}
        <!-- Sanitized with DOMPurify -->
        {@html sanitizeHtml(target.comment)}
      {:else}
        <p class="hint">{tr.userMenu.loading}</p>
      {/if}
    </div>
    <div class="actions"><button type="button" onclick={close}>{tr.userMenu.close}</button></div>
  {:else if d?.kind === 'info'}
    <h2 id="user-dialog-title">{tr.userMenu.infoTitle(target ? displayName(target) : '')}</h2>
    {#if stats}
      <dl>
        <dt>{tr.userMenu.infoVersion}</dt>
        <dd>{[stats.release, stats.version].filter(Boolean).join(' — ') || '—'}</dd>
        <dt>{tr.userMenu.infoOs}</dt>
        <dd>{[stats.os, stats.osVersion].filter(Boolean).join(' ') || '—'}</dd>
        <dt>{tr.userMenu.infoOnline}</dt>
        <dd>{duration(stats.onlineSecs)}</dd>
        <dt>{tr.userMenu.infoIdle}</dt>
        <dd>{duration(stats.idleSecs)}</dd>
        {#if stats.address}
          <dt>{tr.userMenu.infoAddress}</dt>
          <dd>{stats.address}</dd>
        {/if}
        <dt>{tr.userMenu.infoPing}</dt>
        <dd>{stats.tcpPingAvg.toFixed(1)} ms / {stats.udpPingAvg.toFixed(1)} ms</dd>
        {#if stats.bandwidth}
          <dt>{tr.userMenu.infoBandwidth}</dt>
          <dd>{(stats.bandwidth / 1000).toFixed(1)} kbit/s</dd>
        {/if}
        <dt>{tr.userMenu.infoCert}</dt>
        <dd>
          {stats.certificates === 0
            ? tr.userMenu.infoCertNone
            : stats.strongCertificate
              ? tr.userMenu.infoCertStrong
              : tr.userMenu.infoCertSelf}
        </dd>
        <dt>{tr.userMenu.infoOpus}</dt>
        <dd>{stats.opus ? tr.userMenu.yes : tr.userMenu.no}</dd>
        <dt>{tr.userMenu.infoPackets}</dt>
        <dd>
          {tr.userMenu.infoFromClient}: {packets(stats.fromClient)}<br />
          {tr.userMenu.infoFromServer}: {packets(stats.fromServer)}
        </dd>
      </dl>
    {:else}
      <p class="hint">{tr.userMenu.loading}</p>
    {/if}
    <div class="actions">
      {#if d}<button type="button" onclick={() => d.kind === 'info' && userCommand('userStats', d.session)}>↻</button>{/if}
      <button type="button" onclick={close}>{tr.userMenu.close}</button>
    </div>
  {/if}
</dialog>

<style>
  dialog {
    width: min(100% - 2 * var(--gutter), 440px);
    max-height: calc(100% - 2 * var(--gutter));
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg-elev);
    color: var(--text);
    padding: 1.25rem;
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.5);
  }
  h2 {
    margin: 0 0 1rem;
    font-size: 1.05rem;
    overflow-wrap: anywhere;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    margin-bottom: 0.6rem;
  }
  textarea {
    resize: vertical;
  }
  .hint {
    color: var(--text-dim);
    font-size: 0.85rem;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }
  button.danger {
    background: var(--danger);
    border-color: var(--danger);
    color: #fff;
  }
  .comment {
    overflow-wrap: anywhere;
    max-height: 50vh;
    overflow: auto;
  }
  .comment :global(img) {
    max-width: 100%;
  }
  .comment :global(a) {
    color: var(--accent);
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.35rem 0.9rem;
    margin: 0;
    font-size: 0.9rem;
  }
  dt {
    color: var(--text-dim);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
</style>
