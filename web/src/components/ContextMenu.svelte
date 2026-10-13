<script lang="ts">
  import { tick } from 'svelte'
  import { tr } from '../i18n/tr'
  import { joinChannel, toggleDeaf, toggleMute } from '../lib/client'
  import { session } from '../lib/stores/session.svelte'
  import { closeMenu, ui } from '../lib/stores/ui.svelte'
  import { setVolumePref, volumePref } from '../lib/stores/volumes.svelte'
  import Icon from './Icon.svelte'

  let el: HTMLDivElement | undefined = $state()
  let pos = $state({ left: 0, top: 0 })

  const menu = $derived(ui.menu)
  const user = $derived(menu?.kind === 'user' ? session.users.get(menu.id) : undefined)
  const channel = $derived(menu?.kind === 'channel' ? session.channels.get(menu.id) : undefined)
  const isSelf = $derived(user !== undefined && user.session === session.selfSession)
  const pref = $derived(user ? volumePref(user.name) : { volume: 1, muted: false })
  const percent = $derived(Math.round(pref.volume * 100))

  // Keep the menu inside the viewport and focus its first control.
  $effect(() => {
    if (!menu) return
    pos = { left: menu.x, top: menu.y }
    void tick().then(() => {
      if (!el) return
      const r = el.getBoundingClientRect()
      const margin = 8
      pos = {
        left: Math.max(margin, Math.min(menu.x, window.innerWidth - r.width - margin)),
        top: Math.max(margin, Math.min(menu.y, window.innerHeight - r.height - margin)),
      }
      el.querySelector<HTMLElement>('input, button:not(:disabled)')?.focus()
    })
  })

  // The target disappeared (user left, channel removed): close.
  $effect(() => {
    if (menu && !user && !channel) closeMenu()
  })

  function onPointerDown(e: PointerEvent) {
    if (menu && el && !el.contains(e.target as Node)) closeMenu()
  }

  function onKey(e: KeyboardEvent) {
    if (menu && e.key === 'Escape') {
      e.preventDefault()
      closeMenu()
    }
  }

  function setVolume(e: Event) {
    if (user) setVolumePref(user.name, { ...pref, volume: Number((e.currentTarget as HTMLInputElement).value) / 100 })
  }

  function act(fn: () => void) {
    fn()
    closeMenu()
  }
</script>

<svelte:window
  onpointerdowncapture={onPointerDown}
  onkeydown={onKey}
  onresize={closeMenu}
  onblur={closeMenu}
/>

{#if menu && (user || channel)}
  <div
    bind:this={el}
    class="menu"
    role="menu"
    tabindex="-1"
    aria-label={user?.name ?? channel?.name}
    style:left="{pos.left}px"
    style:top="{pos.top}px"
    oncontextmenu={(e) => e.preventDefault()}
  >
    {#if user}
      <div class="title"><Icon name="user" size={14} /> {user.name}{#if isSelf}&nbsp;<span class="dim">{tr.channels.you}</span>{/if}</div>
      {#if isSelf}
        <button role="menuitem" onclick={() => act(toggleMute)}>
          <Icon name={user.selfMute ? 'mic' : 'micOff'} size={16} />
          {user.selfMute ? tr.controls.unmute : tr.controls.mute}
        </button>
        <button role="menuitem" onclick={() => act(toggleDeaf)}>
          <Icon name={user.selfDeaf ? 'headphones' : 'headphonesOff'} size={16} />
          {user.selfDeaf ? tr.controls.undeafen : tr.controls.deafen}
        </button>
      {:else}
        <label class="slider">
          <span>{tr.channels.volume}: {pref.muted ? tr.channels.mutedForMe : `${percent}%`}</span>
          <input type="range" min="0" max="200" step="5" value={percent} oninput={setVolume} disabled={pref.muted} />
        </label>
        <button
          role="menuitemcheckbox"
          aria-checked={pref.muted}
          onclick={() => setVolumePref(user.name, { ...pref, muted: !pref.muted })}
        >
          <span class="box" class:checked={pref.muted} aria-hidden="true">{pref.muted ? '✓' : ''}</span>
          {tr.channels.localMute}
        </button>
        <button
          role="menuitem"
          disabled={!pref.muted && pref.volume === 1}
          onclick={() => setVolumePref(user.name, { volume: 1, muted: false })}
        >
          {tr.channels.resetVolume}
        </button>
      {/if}
    {:else if channel}
      <div class="title"><Icon name="channel" size={14} /> {channel.name}</div>
      <button
        role="menuitem"
        disabled={session.self?.channelId === channel.id}
        onclick={() => act(() => joinChannel(channel.id))}
      >
        {tr.channels.join}
      </button>
    {/if}
  </div>
{/if}

<style>
  .menu {
    position: fixed;
    z-index: 50;
    min-width: 220px;
    max-width: min(300px, calc(100vw - 16px));
    padding: 0.35rem;
    background: var(--bg-elev);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.35);
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    font-size: 0.9rem;
  }
  .title {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.35rem 0.5rem 0.45rem;
    font-weight: 600;
    border-bottom: 1px solid var(--border);
    margin-bottom: 0.2rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dim {
    color: var(--text-dim);
    font-weight: 400;
  }
  button {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    text-align: left;
    border: 0;
    border-radius: 6px;
    background: transparent;
    padding: 0.45rem 0.5rem;
    cursor: pointer;
  }
  button:hover:not(:disabled) {
    background: var(--bg-hover);
  }
  .box {
    display: inline-grid;
    place-items: center;
    width: 16px;
    height: 16px;
    border: 1px solid var(--text-dim);
    border-radius: 4px;
    font-size: 0.7rem;
    line-height: 1;
  }
  .box.checked {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-text);
  }
  .slider {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    padding: 0.35rem 0.5rem;
  }
  input[type='range'] {
    width: 100%;
    accent-color: var(--accent);
  }
</style>
