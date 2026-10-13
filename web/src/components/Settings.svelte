<script lang="ts">
  import { tr } from '../i18n/tr'
  import { listDevices, restartMicrophone } from '../lib/audio/transmitter'
  import { keyName } from '../lib/keys'
  import { audioSettings } from '../lib/stores/audioSettings.svelte'

  let { open = $bindable(false) }: { open: boolean } = $props()

  let dialog: HTMLDialogElement | undefined = $state()
  let inputs = $state<MediaDeviceInfo[]>([])
  let capturingKey = $state(false)

  $effect(() => {
    if (!dialog) return
    if (open && !dialog.open) {
      dialog.showModal()
      void refreshDevices()
    } else if (!open && dialog.open) {
      dialog.close()
    }
  })

  async function refreshDevices() {
    try {
      inputs = (await listDevices()).inputs
    } catch {
      inputs = []
    }
  }

  async function chooseInput(e: Event) {
    audioSettings.update({ inputDeviceId: (e.currentTarget as HTMLSelectElement).value })
    await restartMicrophone()
  }

  function captureKey(e: KeyboardEvent) {
    if (!capturingKey) return
    e.preventDefault()
    e.stopPropagation()
    if (e.code !== 'Escape') audioSettings.update({ pttKey: e.code })
    capturingKey = false
  }
</script>

<svelte:window onkeydowncapture={captureKey} />

<dialog bind:this={dialog} onclose={() => (open = false)} aria-labelledby="settings-title">
  <header>
    <h2 id="settings-title">{tr.settings.title}</h2>
    <button type="button" onclick={() => (open = false)}>{tr.settings.close}</button>
  </header>

  <label class="field">
    <span>{tr.settings.input}</span>
    <select value={audioSettings.prefs.inputDeviceId} onchange={chooseInput}>
      <option value="">{tr.settings.defaultDevice}</option>
      {#each inputs as d (d.deviceId)}
        {#if d.deviceId && d.deviceId !== 'default'}
          <option value={d.deviceId}>{d.label || d.deviceId.slice(0, 8)}</option>
        {/if}
      {/each}
    </select>
  </label>

  <div class="field">
    <span>{tr.settings.pttKey}</span>
    <div class="row">
      <kbd>{capturingKey ? tr.settings.pttKeyPress : keyName(audioSettings.prefs.pttKey)}</kbd>
      <button type="button" onclick={() => (capturingKey = true)} disabled={capturingKey}>{tr.settings.pttKeyChange}</button>
    </div>
  </div>
</dialog>

<style>
  dialog {
    width: min(100% - 2 * var(--gutter), 460px);
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg-elev);
    color: var(--text);
    padding: 1.25rem;
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.5);
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }
  h2 {
    margin: 0;
    font-size: 1.15rem;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    margin-bottom: 1rem;
  }
  select {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 0.45rem;
  }
  .row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  kbd {
    font: inherit;
    padding: 0.35rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg);
    min-width: 6rem;
  }
</style>
