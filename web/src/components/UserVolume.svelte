<script lang="ts">
  import { tr } from '../i18n/tr'
  import { setVolumePref, volumePref } from '../lib/stores/volumes.svelte'

  let { name }: { name: string } = $props()

  const pref = $derived(volumePref(name))
  const percent = $derived(Math.round(pref.volume * 100))

  function setVolume(e: Event) {
    setVolumePref(name, { ...pref, volume: Number((e.currentTarget as HTMLInputElement).value) / 100 })
  }
  function setMuted(e: Event) {
    setVolumePref(name, { ...pref, muted: (e.currentTarget as HTMLInputElement).checked })
  }
</script>

<div class="panel">
  <label class="slider">
    <span>{tr.channels.volume}: {percent}%</span>
    <input type="range" min="0" max="200" step="5" value={percent} oninput={setVolume} disabled={pref.muted} />
  </label>
  <div class="row">
    <label><input type="checkbox" checked={pref.muted} onchange={setMuted} /> {tr.channels.localMute}</label>
    <button type="button" onclick={() => setVolumePref(name, { volume: 1, muted: false })}>{tr.channels.resetVolume}</button>
  </div>
</div>

<style>
  .panel {
    margin: 0.15rem 0.5rem 0.4rem calc(1.6rem + var(--depth) * 1rem);
    padding: 0.6rem;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-elev);
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    font-size: 0.875rem;
  }
  .slider {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  input[type='range'] {
    width: 100%;
    accent-color: var(--accent);
  }
  .row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
  }
  button {
    padding: 0.2rem 0.6rem;
    font-size: 0.8rem;
  }
</style>
