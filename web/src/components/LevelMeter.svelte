<script lang="ts">
  import { tr } from '../i18n/tr'
  import { rmsToMeter } from '../lib/audio/frames'
  import { audioSettings } from '../lib/stores/audioSettings.svelte'
  import { mic } from '../lib/stores/mic.svelte'

  const level = $derived(rmsToMeter(mic.level))
  const vad = $derived(audioSettings.prefs.mode === 'vad')
  const threshold = $derived(audioSettings.prefs.vadThreshold)
  const above = $derived(level >= threshold)
</script>

<div class="meter" class:vad title={vad ? tr.controls.threshold : tr.controls.level}>
  <div class="fill" class:above={vad && above} style:width="{level * 100}%"></div>
  {#if vad}
    <div class="mark" style:left="{threshold * 100}%"></div>
    <input
      type="range"
      min="0"
      max="1"
      step="0.01"
      value={threshold}
      aria-label={tr.controls.threshold}
      oninput={(e) => audioSettings.update({ vadThreshold: Number(e.currentTarget.value) })}
    />
  {/if}
</div>

<style>
  .meter {
    position: relative;
    height: 10px;
    min-width: 120px;
    flex: 1;
    max-width: 260px;
    border-radius: 5px;
    background: var(--bg-active);
    overflow: hidden;
  }
  .fill {
    position: absolute;
    inset: 0 auto 0 0;
    background: var(--text-dim);
    transition: width 60ms linear;
  }
  .fill.above {
    background: var(--speaking);
  }
  .mark {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 2px;
    margin-left: -1px;
    background: var(--warn);
  }
  input[type='range'] {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    opacity: 0;
    cursor: ew-resize;
  }
</style>
