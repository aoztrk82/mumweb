<script lang="ts">
  import { tr } from '../i18n/tr'
  import { createChannel } from '../lib/client'
  import { session } from '../lib/stores/session.svelte'
  import { ui } from '../lib/stores/ui.svelte'

  let dialog: HTMLDialogElement | undefined = $state()
  let name = $state('')
  let temporary = $state(false)

  const parent = $derived(ui.createIn === null ? undefined : session.channels.get(ui.createIn))

  $effect(() => {
    if (!dialog) return
    if (ui.createIn !== null && !dialog.open) {
      name = ''
      temporary = false
      dialog.showModal()
    } else if (ui.createIn === null && dialog.open) {
      dialog.close()
    }
  })

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (ui.createIn === null || !name.trim()) return
    createChannel(ui.createIn, name, temporary)
    ui.createIn = null
  }
</script>

<dialog bind:this={dialog} onclose={() => (ui.createIn = null)} aria-labelledby="create-title">
  <form onsubmit={submit}>
    <h2 id="create-title">{tr.channels.createTitle(parent?.name ?? '')}</h2>
    <label class="field">
      <span>{tr.channels.createName}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input type="text" bind:value={name} maxlength="100" required autofocus />
    </label>
    <label class="check">
      <input type="checkbox" bind:checked={temporary} />
      {tr.channels.createTemporary}
    </label>
    <p class="hint">{tr.channels.createHint}</p>
    <div class="actions">
      <button type="button" onclick={() => (ui.createIn = null)}>{tr.channels.cancel}</button>
      <button type="submit" class="primary" disabled={!name.trim()}>{tr.channels.createSubmit}</button>
    </div>
  </form>
</dialog>

<style>
  dialog {
    width: min(100% - 2 * var(--gutter), 400px);
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
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    margin-bottom: 0.8rem;
  }
  .check {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .hint {
    color: var(--text-dim);
    font-size: 0.85rem;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }
</style>
