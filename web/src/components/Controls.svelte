<script lang="ts">
  import { tr } from '../i18n/tr'
  import { disconnect, toggleDeaf, toggleMute } from '../lib/client'
  import { session } from '../lib/stores/session.svelte'
  import Icon from './Icon.svelte'

  const muted = $derived(session.self?.selfMute ?? false)
  const deafened = $derived(session.self?.selfDeaf ?? false)
</script>

<div class="controls">
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
  <span class="spacer"></span>
  <button onclick={disconnect} title={tr.controls.disconnect}>
    <Icon name="logout" />
    <span class="label">{tr.controls.disconnect}</span>
  </button>
</div>

<style>
  .controls {
    display: flex;
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
  .spacer {
    flex: 1;
  }
  @media (max-width: 640px) {
    .label {
      display: none;
    }
  }
</style>
