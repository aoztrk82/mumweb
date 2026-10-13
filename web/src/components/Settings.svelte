<script lang="ts">
  import { t } from '../i18n/index.svelte'
  import { canSelectOutput, setOutputDevice } from '../lib/audio/context'
  import { listDevices, restartMicrophone } from '../lib/audio/transmitter'
  import { clearCert, loadCert, saveCert } from '../lib/certStore'
  import { certToPem, downloadText, parsePemBundle } from '../lib/certFile'
  import { reconnect, syncTokens } from '../lib/client'
  import { tokens } from '../lib/stores/tokens.svelte'
  import { keyName } from '../lib/keys'
  import { audioSettings } from '../lib/stores/audioSettings.svelte'
  import { prefs } from '../lib/stores/prefs.svelte'
  import { session } from '../lib/stores/session.svelte'
  import LevelMeter from './LevelMeter.svelte'

  let { open = $bindable(false) }: { open: boolean } = $props()

  let dialog: HTMLDialogElement | undefined = $state()
  let fileInput: HTMLInputElement | undefined = $state()
  let inputs = $state<MediaDeviceInfo[]>([])
  let outputs = $state<MediaDeviceInfo[]>([])
  let capturingKey = $state(false)
  let hasCert = $state(false)
  let certMessage = $state<{ text: string; error?: boolean; reconnect?: boolean } | null>(null)

  const p = $derived(audioSettings.prefs)

  $effect(() => {
    if (!dialog) return
    if (open && !dialog.open) {
      dialog.showModal()
      certMessage = null
      void refresh()
    } else if (!open && dialog.open) {
      dialog.close()
    }
  })

  async function refresh() {
    try {
      const d = await listDevices()
      inputs = d.inputs.filter((x) => x.deviceId && x.deviceId !== 'default')
      outputs = d.outputs.filter((x) => x.deviceId && x.deviceId !== 'default')
    } catch {
      inputs = outputs = []
    }
    hasCert = (await loadCert()) !== null
  }

  async function chooseInput(e: Event) {
    audioSettings.update({ inputDeviceId: (e.currentTarget as HTMLSelectElement).value })
    await restartMicrophone()
  }

  async function chooseOutput(e: Event) {
    const id = (e.currentTarget as HTMLSelectElement).value
    audioSettings.update({ outputDeviceId: id })
    await setOutputDevice(id)
  }

  async function toggleNoise(e: Event) {
    audioSettings.update({ noiseSuppression: (e.currentTarget as HTMLInputElement).checked })
    await restartMicrophone()
  }

  function captureKey(e: KeyboardEvent) {
    if (!capturingKey) return
    e.preventDefault()
    e.stopPropagation()
    if (e.code !== 'Escape') audioSettings.update({ pttKey: e.code })
    capturingKey = false
  }

  async function exportCert() {
    const c = await loadCert()
    if (c) downloadText(`mumweb-${session.self?.name ?? 'kimlik'}.pem`, certToPem(c))
  }

  async function importCert(e: Event) {
    const file = (e.currentTarget as HTMLInputElement).files?.[0]
    ;(e.currentTarget as HTMLInputElement).value = ''
    if (!file) return
    const pair = parsePemBundle(await file.text())
    if (!pair) {
      certMessage = { text: t.settings.certImportInvalid, error: true }
      return
    }
    await saveCert(pair)
    hasCert = true
    certMessage = { text: t.settings.certImported, reconnect: session.joined }
  }

  async function resetCert() {
    if (!confirm(t.settings.certResetConfirm)) return
    await clearCert()
    hasCert = false
    certMessage = { text: t.settings.certResetDone, reconnect: session.joined }
  }
</script>

<svelte:window onkeydowncapture={captureKey} />

<dialog bind:this={dialog} onclose={() => (open = false)} aria-labelledby="settings-title">
  <header>
    <h2 id="settings-title">{t.settings.title}</h2>
    <button type="button" onclick={() => (open = false)}>{t.settings.close}</button>
  </header>

  <section>
    <h3>{t.settings.sectionAudio}</h3>
    <label class="field">
      <span>{t.settings.input}</span>
      <select value={p.inputDeviceId} onchange={chooseInput}>
        <option value="">{t.settings.defaultDevice}</option>
        {#each inputs as d (d.deviceId)}<option value={d.deviceId}>{d.label || d.deviceId.slice(0, 8)}</option>{/each}
      </select>
    </label>

    <label class="field">
      <span>{t.settings.output}</span>
      {#if canSelectOutput}
        <select value={p.outputDeviceId} onchange={chooseOutput}>
          <option value="">{t.settings.defaultDevice}</option>
          {#each outputs as d (d.deviceId)}<option value={d.deviceId}>{d.label || d.deviceId.slice(0, 8)}</option>{/each}
        </select>
      {:else}
        <small>{t.settings.outputUnsupported}</small>
      {/if}
    </label>

    <label class="check">
      <input type="checkbox" checked={p.noiseSuppression} onchange={toggleNoise} />
      {t.settings.noiseSuppression}
    </label>

    <div class="field">
      <span>{t.settings.mode}</span>
      <div class="segmented" role="radiogroup" aria-label={t.settings.mode}>
        <label><input type="radio" name="mode" checked={p.mode === 'ptt'} onchange={() => audioSettings.update({ mode: 'ptt' })} /> {t.controls.modePtt}</label>
        <label><input type="radio" name="mode" checked={p.mode === 'vad'} onchange={() => audioSettings.update({ mode: 'vad' })} /> {t.controls.modeVad}</label>
      </div>
    </div>

    {#if p.mode === 'ptt'}
      <div class="field">
        <span>{t.settings.pttKey}</span>
        <div class="row">
          <kbd>{capturingKey ? t.settings.pttKeyPress : keyName(p.pttKey)}</kbd>
          <button type="button" onclick={() => (capturingKey = true)} disabled={capturingKey}>{t.settings.pttKeyChange}</button>
        </div>
      </div>
    {:else}
      <div class="field">
        <span>{t.settings.vadThreshold}</span>
        <div class="meter-row"><LevelMeter /></div>
        <small>{t.settings.vadHint}</small>
      </div>
    {/if}
  </section>

  <section>
    <h3>{t.settings.sectionAppearance}</h3>
    <div class="field">
      <span>{t.settings.theme}</span>
      <div class="segmented" role="radiogroup" aria-label={t.settings.theme}>
        <label><input type="radio" name="theme" checked={prefs.value.theme === 'dark'} onchange={() => prefs.update({ theme: 'dark' })} /> {t.settings.themeDark}</label>
        <label><input type="radio" name="theme" checked={prefs.value.theme === 'light'} onchange={() => prefs.update({ theme: 'light' })} /> {t.settings.themeLight}</label>
      </div>
    </div>
    <label class="check">
      <input type="checkbox" checked={prefs.value.sounds} onchange={(e) => prefs.update({ sounds: e.currentTarget.checked })} />
      {t.settings.sounds}
    </label>
  </section>

  <section>
    <h3>{t.settings.sectionTokens}</h3>
    <p class="help">{t.settings.tokensHelp}</p>
    {#if tokens.remembered.size === 0}
      <p class="help">{t.settings.tokensNone}</p>
    {:else}
      <ul class="tokens">
        {#each [...tokens.remembered] as tk (tk)}
          <li>
            <code>{tk.length > 2 ? tk.slice(0, 2) + '•'.repeat(Math.min(8, tk.length - 2)) : '••'}</code>
            <button type="button" onclick={() => { tokens.remove(tk); syncTokens() }}>{t.settings.tokenRemove}</button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <section>
    <h3>{t.settings.sectionIdentity}</h3>
    <p class="help">{t.settings.certHelp}</p>
    <p class="help">{hasCert ? t.settings.certPresent : t.settings.certNone}</p>
    <div class="row wrap">
      <button type="button" onclick={exportCert} disabled={!hasCert}>{t.settings.certExport}</button>
      <button type="button" onclick={() => fileInput?.click()}>{t.settings.certImport}</button>
      <button type="button" class="danger" onclick={resetCert} disabled={!hasCert}>{t.settings.certReset}</button>
      <input bind:this={fileInput} type="file" accept=".pem,.crt,.key,.txt,application/x-pem-file" hidden onchange={importCert} />
    </div>
    {#if certMessage}
      <p class="msg" class:error={certMessage.error} role="status">
        {certMessage.text}
        {#if certMessage.reconnect}
          <button type="button" onclick={() => { reconnect(); open = false }}>{t.settings.reconnectNow}</button>
        {/if}
      </p>
    {/if}
  </section>
</dialog>

<style>
  dialog {
    width: min(100% - 2 * var(--gutter), 520px);
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
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
  }
  h2 {
    margin: 0;
    font-size: 1.15rem;
  }
  h3 {
    margin: 1.1rem 0 0.6rem;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-dim);
  }
  section + section {
    border-top: 1px solid var(--border);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    margin-bottom: 0.9rem;
  }
  .check {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    margin-bottom: 0.9rem;
  }
  select {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 0.45rem;
    max-width: 100%;
  }
  small,
  .help {
    color: var(--text-dim);
    font-size: 0.85rem;
  }
  .help {
    margin: 0 0 0.6rem;
  }
  .row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .wrap {
    flex-wrap: wrap;
  }
  .segmented {
    display: flex;
    gap: 1rem;
  }
  .segmented label {
    display: flex;
    gap: 0.35rem;
    align-items: center;
  }
  .meter-row {
    display: flex;
  }
  .meter-row :global(.meter) {
    max-width: none;
    height: 16px;
  }
  kbd {
    font: inherit;
    padding: 0.35rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg);
    min-width: 6rem;
  }
  .danger {
    color: var(--danger);
  }
  .tokens {
    list-style: none;
    margin: 0 0 0.75rem;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .tokens li {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .tokens button {
    padding: 0.2rem 0.6rem;
    font-size: 0.8rem;
  }
  .msg {
    margin: 0.75rem 0 0;
    font-size: 0.9rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
  }
  .msg.error {
    color: var(--danger);
  }
</style>
