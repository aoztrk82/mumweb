<script lang="ts">
  import { tr } from '../i18n/tr'
  import { setPushToTalk } from '../lib/audio/transmitter'
  import { disconnect, toggleDeaf, toggleMute } from '../lib/client'
  import { isTyping, keyName } from '../lib/keys'
  import { audioSettings } from '../lib/stores/audioSettings.svelte'
  import { mic } from '../lib/stores/mic.svelte'
  import { session } from '../lib/stores/session.svelte'
  import Icon from './Icon.svelte'
  import LevelMeter from './LevelMeter.svelte'

  let { onSettings }: { onSettings: () => void } = $props()

  const muted = $derived(session.self?.selfMute ?? false)
  const deafened = $derived(session.self?.selfDeaf ?? false)
  const mode = $derived(audioSettings.prefs.mode)
  const talking = $derived(session.selfSession !== null && session.speaking.has(session.selfSession))
  let pttDown = $state(false)

  function setPtt(on: boolean) {
    pttDown = on
    setPushToTalk(on)
  }

  function pttPointerDown(e: PointerEvent) {
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
    setPtt(true)
  }

  // Keyboard push-to-talk (ignored while typing in a text field).
  $effect(() => {
    const key = audioSettings.prefs.pttKey
    const down = (e: KeyboardEvent) => {
      if (e.code !== key || e.repeat || isTyping(e) || audioSettings.prefs.mode !== 'ptt') return
      e.preventDefault()
      setPtt(true)
    }
    const up = (e: KeyboardEvent) => {
      if (e.code === key) setPtt(false)
    }
    const release = () => setPtt(false)
    window.addEventListener('keydown', down)
    window.addEventListener('keyup', up)
    window.addEventListener('blur', release)
    return () => {
      window.removeEventListener('keydown', down)
      window.removeEventListener('keyup', up)
      window.removeEventListener('blur', release)
      setPtt(false)
    }
  })
</script>

{#if mic.state === 'denied' || mic.state === 'unavailable'}
  <div class="mic-warning" role="alert">{mic.state === 'denied' ? tr.audio.micDenied : tr.audio.micUnavailable}</div>
{/if}

{#if mic.state === 'on' && mode === 'ptt'}
  <!-- Large push-to-talk button for touch screens (shown at <= 640 px). -->
  <button
    class="ptt-big"
    class:talking={pttDown}
    disabled={!session.ready}
    onpointerdown={pttPointerDown}
    onpointerup={() => setPtt(false)}
    onpointercancel={() => setPtt(false)}
    oncontextmenu={(e) => e.preventDefault()}
  >
    <Icon name="mic" size={22} />
    {pttDown ? tr.controls.talking : tr.controls.holdToTalk}
  </button>
{/if}

<div class="controls" class:ptt-mode={mode === 'ptt'}>
  <button
    class:on={muted}
    onclick={toggleMute}
    disabled={!session.ready}
    aria-pressed={muted}
    title={muted ? tr.controls.unmute : tr.controls.mute}
  >
    <Icon name={muted ? 'micOff' : 'mic'} />
    <span class="label">{muted ? tr.controls.unmute : tr.controls.mute}</span>
  </button>
  <button
    class:on={deafened}
    onclick={toggleDeaf}
    disabled={!session.ready}
    aria-pressed={deafened}
    title={deafened ? tr.controls.undeafen : tr.controls.deafen}
  >
    <Icon name={deafened ? 'headphonesOff' : 'headphones'} />
    <span class="label">{deafened ? tr.controls.undeafen : tr.controls.deafen}</span>
  </button>

  {#if mic.state === 'on'}
    <div class="mode" role="radiogroup" aria-label="Konuşma modu">
      <button role="radio" aria-checked={mode === 'ptt'} class:active={mode === 'ptt'} onclick={() => audioSettings.update({ mode: 'ptt' })}>
        {tr.controls.modePtt}
      </button>
      <button role="radio" aria-checked={mode === 'vad'} class:active={mode === 'vad'} onclick={() => audioSettings.update({ mode: 'vad' })}>
        {tr.controls.modeVad}
      </button>
    </div>
    <LevelMeter />
    {#if mode === 'ptt'}
      <button
        class="ptt"
        class:talking={pttDown}
        title={tr.controls.pttKeyHint(keyName(audioSettings.prefs.pttKey))}
        onpointerdown={pttPointerDown}
        onpointerup={() => setPtt(false)}
        onpointercancel={() => setPtt(false)}
        oncontextmenu={(e) => e.preventDefault()}
      >
        {pttDown ? tr.controls.talking : tr.controls.holdToTalk}
        <kbd>{keyName(audioSettings.prefs.pttKey)}</kbd>
      </button>
    {:else if talking}
      <span class="talking-label">{tr.controls.talking}</span>
    {/if}
  {/if}

  <span class="spacer"></span>
  <button onclick={onSettings} title={tr.controls.settings}>
    <Icon name="settings" />
    <span class="label">{tr.controls.settings}</span>
  </button>
  <button onclick={disconnect} title={tr.controls.disconnect}>
    <Icon name="logout" />
    <span class="label">{tr.controls.disconnect}</span>
  </button>
</div>

<style>
  .controls {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
    padding: 0.6rem var(--gutter);
    border-top: 1px solid var(--border);
    background: var(--bg-elev);
  }
  button {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }
  button.on {
    color: var(--danger);
    border-color: var(--danger);
  }
  .mode {
    display: inline-flex;
  }
  .mode button {
    border-radius: 0;
    font-size: 0.85rem;
    white-space: nowrap;
  }
  .mode button:first-child {
    border-radius: var(--radius) 0 0 var(--radius);
  }
  .mode button:last-child {
    border-radius: 0 var(--radius) var(--radius) 0;
    border-left: 0;
  }
  .mode button.active {
    background: var(--bg-active);
    font-weight: 600;
  }
  .ptt {
    user-select: none;
    touch-action: none;
  }
  .ptt.talking {
    background: var(--speaking);
    border-color: var(--speaking);
    color: #08130c;
  }
  kbd {
    font: inherit;
    font-size: 0.75rem;
    padding: 0 0.3rem;
    border: 1px solid currentColor;
    border-radius: 4px;
    opacity: 0.7;
  }
  .talking-label {
    color: var(--speaking);
    font-size: 0.85rem;
  }
  .spacer {
    flex: 1;
  }
  .ptt-big {
    display: none;
  }
  .mic-warning {
    padding: 0.45rem var(--gutter);
    font-size: 0.85rem;
    color: var(--warn);
    border-top: 1px solid var(--border);
  }
  @media (max-width: 900px) {
    .label {
      display: none;
    }
  }
  @media (max-width: 640px) {
    .controls {
      flex-wrap: nowrap;
      gap: 0.3rem;
    }
    .label,
    .ptt,
    .talking-label {
      display: none;
    }
    .controls > button {
      padding: 0.5rem 0.6rem;
    }
    .mode button {
      padding: 0.45rem 0.4rem;
      font-size: 0.8rem;
    }
    .controls :global(.meter) {
      min-width: 20px;
    }
    .ptt-mode :global(.meter) {
      display: none;
    }
    .ptt-big {
      display: flex;
      justify-content: center;
      align-items: center;
      gap: 0.6rem;
      width: 100%;
      min-height: 72px;
      border: 0;
      border-top: 1px solid var(--border);
      border-radius: 0;
      font-size: 1.1rem;
      font-weight: 600;
      background: var(--bg-active);
      user-select: none;
      -webkit-user-select: none;
      touch-action: none;
      -webkit-touch-callout: none;
    }
    .ptt-big.talking {
      background: var(--speaking);
      color: #08130c;
    }
  }
</style>
