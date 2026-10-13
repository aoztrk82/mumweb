<script lang="ts">
  import { tick } from 'svelte'
  import { tr } from '../i18n/tr'
  import {
    banUser,
    joinChannel,
    kickUser,
    linkChannels,
    moveUser,
    queryPermissions,
    removeChannel,
    sendChannelText,
    sendPrivateText,
    setListening,
    setUser,
    toggleDeaf,
    toggleMute,
    userCommand,
  } from '../lib/client'
  import { channelUrl } from '../lib/channelPath'
  import { channelPerms, has, Perm, rootPerms } from '../lib/permissions'
  import { channelFilter } from '../lib/stores/channelFilter.svelte'
  import { session } from '../lib/stores/session.svelte'
  import { closeMenu, ui } from '../lib/stores/ui.svelte'
  import { displayName, updateUserPref, userPref } from '../lib/stores/userPrefs.svelte'
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
  const uPref = $derived(user ? userPref(user) : {})
  const name = $derived(user ? displayName(user) : '')
  // Permissions as the desktop client evaluates them: target user's channel and root.
  const p = $derived(user ? channelPerms(user.channelId, queryPermissions) : 0)
  const root = $derived(user ? rootPerms() : 0)
  // Channel menu (MainWindow::qmChannel_aboutToShow + updateMenuPermissions).
  const home = $derived(session.self ? session.channels.get(session.self.channelId) : undefined)
  const cp = $derived(channel ? channelPerms(channel.id, queryPermissions) : 0)
  const homep = $derived(home ? channelPerms(home.id, queryPermissions) : 0)
  const isHome = $derived(!!channel && channel.id === home?.id)
  const linkedToHome = $derived(!!channel && !!home && home.links.includes(channel.id))
  const listening = $derived(!!channel && !!session.self?.listening.includes(channel.id))
  const otherChannel = $derived(!!user && !isSelf && !!session.self && session.self.channelId !== user.channelId)

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

  function askReason(kind: 'kick' | 'ban') {
    if (!user) return
    const target = user.session
    const n = name
    ui.dialog = {
      kind: 'prompt',
      title: kind === 'kick' ? tr.userMenu.kickTitle(n) : tr.userMenu.banTitle(n),
      label: tr.userMenu.reason,
      value: '',
      hint: kind === 'ban' ? tr.userMenu.banHint : undefined,
      confirm: kind === 'kick' ? tr.userMenu.kickConfirm : tr.userMenu.banConfirm,
      danger: true,
      allowEmpty: true,
      onSubmit: (reason) => (kind === 'kick' ? kickUser(target, reason) : banUser(target, reason)),
    }
  }

  function askNickname() {
    if (!user) return
    const u = user
    ui.dialog = {
      kind: 'prompt',
      title: tr.userMenu.nicknameTitle(u.name),
      label: tr.userMenu.nicknameLabel,
      value: userPref(u).nickname ?? '',
      confirm: tr.userMenu.save,
      allowEmpty: true,
      onSubmit: (v) => updateUserPref(u, { nickname: v.trim() || undefined }),
    }
  }

  function askMessage() {
    if (!user) return
    const target = user.session
    ui.dialog = {
      kind: 'prompt',
      title: tr.userMenu.messageTitle(name),
      label: tr.userMenu.messageLabel,
      value: '',
      multiline: true,
      confirm: tr.userMenu.send,
      onSubmit: (v) => sendPrivateText(target, v),
    }
  }

  function viewComment() {
    if (!user) return
    if (!user.comment) userCommand('requestComment', user.session)
    ui.dialog = { kind: 'comment', session: user.session }
  }

  function showInfo() {
    if (!user) return
    userCommand('userStats', user.session)
    ui.dialog = { kind: 'info', session: user.session }
  }

  function confirmRemove() {
    if (channel && confirm(tr.channelMenu.removeConfirm(channel.name))) removeChannel(channel.id)
  }

  function askChannelMessage() {
    if (!channel) return
    const id = channel.id
    ui.dialog = {
      kind: 'prompt',
      title: tr.channelMenu.messageTitle(channel.name),
      label: tr.userMenu.messageLabel,
      value: '',
      multiline: true,
      checkbox: tr.channelMenu.messageTree,
      confirm: tr.userMenu.send,
      onSubmit: (v, tree) => sendChannelText(id, v, tree),
    }
  }

  async function copyUrl() {
    if (!channel) return
    const url = channelUrl(channel.id)
    try {
      await navigator.clipboard.writeText(url)
      session.addChat({ kind: 'system', body: tr.chat.linkCopied })
    } catch {
      session.addChat({ kind: 'system', body: tr.chat.linkCopyFailed(url) })
    }
  }

  function register() {
    if (user && confirm(tr.userMenu.registerConfirm(name))) userCommand('register', user.session)
  }
</script>

{#snippet check(label: string, checked: boolean, enabled: boolean, toggle: () => void)}
  <button role="menuitemcheckbox" aria-checked={checked} disabled={!enabled} onclick={() => act(toggle)}>
    <span class="box" class:checked aria-hidden="true">{checked ? '✓' : ''}</span>
    {label}
  </button>
{/snippet}

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
      <div class="title">
        <Icon name="user" size={14} /> {name}{#if isSelf}&nbsp;<span class="dim">{tr.channels.you}</span>{/if}
      </div>

      {#if otherChannel}
        <button role="menuitem" onclick={() => act(() => joinChannel(user.channelId))}>{tr.userMenu.joinUserChannel}</button>
        <button role="menuitem" onclick={() => act(() => moveUser(user.session, session.self!.channelId))}>
          {tr.userMenu.moveToMyChannel}
        </button>
        <hr />
      {/if}

      {#if has(root, Perm.Kick, Perm.Ban, Perm.Write)}
        <button role="menuitem" disabled={isSelf} onclick={() => act(() => askReason('kick'))}>{tr.userMenu.kick}</button>
      {/if}
      {#if has(root, Perm.Ban, Perm.Write)}
        <button role="menuitem" disabled={isSelf} onclick={() => act(() => askReason('ban'))}>{tr.userMenu.ban}</button>
      {/if}
      {@render check(
        tr.userMenu.mute,
        user.mute || user.suppress,
        has(p, Perm.Write, Perm.MuteDeafen) && (!isSelf || user.mute || user.suppress),
        () => setUser(user.session, { mute: !(user.mute || user.suppress) }),
      )}
      {@render check(
        tr.userMenu.deafen,
        user.deaf,
        has(p, Perm.Write, Perm.MuteDeafen) && (!isSelf || user.deaf),
        () => setUser(user.session, { deaf: !user.deaf }),
      )}
      {@render check(tr.userMenu.prioritySpeaker, user.prioritySpeaker, has(p, Perm.Write, Perm.MuteDeafen), () =>
        setUser(user.session, { prioritySpeaker: !user.prioritySpeaker }),
      )}
      {@render check(tr.userMenu.localMute, pref.muted, !isSelf, () =>
        setVolumePref(user.name, { ...pref, muted: !pref.muted }),
      )}
      {@render check(tr.userMenu.ignoreMessages, !!uPref.ignoreMessages, !isSelf, () =>
        updateUserPref(user, { ignoreMessages: !uPref.ignoreMessages }),
      )}

      {#if !isSelf}
        <hr />
        <label class="slider">
          <span>{tr.userMenu.localVolume}: {pref.muted ? tr.channels.mutedForMe : `${percent}%`}</span>
          <input type="range" min="0" max="200" step="5" value={percent} oninput={setVolume} disabled={pref.muted} />
        </label>
        <hr />
      {/if}

      <button role="menuitem" disabled={isSelf} onclick={() => act(askNickname)}>{tr.userMenu.nickname}</button>
      {#if !isSelf}
        <button role="menuitem" disabled={!user.hasComment} onclick={() => act(viewComment)}>{tr.userMenu.viewComment}</button>
        <button
          role="menuitem"
          disabled={!user.hasComment || !has(root, Perm.ResetUserContent, Perm.Write)}
          onclick={() => act(() => userCommand('resetComment', user.session))}
        >
          {tr.userMenu.resetComment}
        </button>
        <button
          role="menuitem"
          disabled={!user.hasTexture || !has(root, Perm.ResetUserContent, Perm.Write)}
          onclick={() => act(() => userCommand('resetTexture', user.session))}
        >
          {tr.userMenu.removeAvatar}
        </button>
      {/if}
      <button role="menuitem" disabled={!has(p, Perm.Write, Perm.TextMessage)} onclick={() => act(askMessage)}>
        {tr.userMenu.sendMessage}
      </button>
      <button
        role="menuitem"
        disabled={!(has(root, Perm.Write, Perm.Register) || has(p, Perm.Write, Perm.Enter) || isSelf)}
        onclick={() => act(showInfo)}
      >
        {tr.userMenu.info}
      </button>

      {#if !user.registered && user.hash && has(root, isSelf ? Perm.SelfRegister : Perm.Register, Perm.Write)}
        <hr />
        <button role="menuitem" onclick={() => act(register)}>{tr.userMenu.register}</button>
      {/if}

      {#if user.hash && (uPref.friend || !isSelf)}
        <hr />
        {#if !uPref.friend}
          <button role="menuitem" onclick={() => act(() => updateUserPref(user, { friend: user.name }))}>
            <Icon name="heart" size={16} />
            {tr.userMenu.addFriend}
          </button>
        {:else}
          {#if uPref.friend !== user.name}
            <button role="menuitem" onclick={() => act(() => updateUserPref(user, { friend: user.name }))}>
              {tr.userMenu.updateFriend}
            </button>
          {/if}
          <button role="menuitem" onclick={() => act(() => updateUserPref(user, { friend: undefined }))}>
            {tr.userMenu.removeFriend}
          </button>
        {/if}
      {/if}

      {#if isSelf}
        <hr />
        <button role="menuitem" onclick={() => act(toggleMute)}>
          <Icon name={user.selfMute ? 'mic' : 'micOff'} size={16} />
          {user.selfMute ? tr.controls.unmute : tr.controls.mute}
        </button>
        <button role="menuitem" onclick={() => act(toggleDeaf)}>
          <Icon name={user.selfDeaf ? 'headphones' : 'headphonesOff'} size={16} />
          {user.selfDeaf ? tr.controls.undeafen : tr.controls.deafen}
        </button>
      {/if}
    {:else if channel}
      <div class="title"><Icon name="channel" size={14} /> {channel.name}</div>
      {#if !isHome}
        <button role="menuitem" disabled={!has(cp, Perm.Write, Perm.Enter)} onclick={() => act(() => joinChannel(channel.id))}>
          {tr.channels.join}
        </button>
      {/if}
      {@render check(tr.channelMenu.listen, listening, true, () => setListening(channel.id, !listening))}
      <hr />
      <button
        role="menuitem"
        disabled={!has(cp, Perm.Write, Perm.MakeChannel, Perm.MakeTempChannel)}
        onclick={() => act(() => (ui.createIn = channel.id))}
      >
        {tr.channelMenu.add}
      </button>
      <button
        role="menuitem"
        disabled={!(has(cp, Perm.Write) || has(rootPerms(), Perm.Write))}
        onclick={() => act(() => (ui.dialog = { kind: 'editChannel', channelId: channel.id }))}
      >
        {tr.channelMenu.edit}
      </button>
      <button role="menuitem" disabled={channel.id === 0 || !has(cp, Perm.Write)} onclick={() => act(confirmRemove)}>
        {tr.channelMenu.remove}
      </button>
      <hr />
      <button
        role="menuitem"
        disabled={isHome || linkedToHome || !has(cp, Perm.Write, Perm.LinkChannel) || !has(homep, Perm.Write, Perm.LinkChannel)}
        onclick={() => act(() => home && linkChannels(home.id, [channel.id], []))}
      >
        {tr.channelMenu.link}
      </button>
      <button
        role="menuitem"
        disabled={isHome || !linkedToHome || !(has(cp, Perm.Write, Perm.LinkChannel) || has(homep, Perm.Write, Perm.LinkChannel))}
        onclick={() => act(() => home && linkChannels(home.id, [], [channel.id]))}
      >
        {tr.channelMenu.unlink}
      </button>
      <button
        role="menuitem"
        disabled={!home || home.links.length === 0 || !has(homep, Perm.Write, Perm.LinkChannel)}
        onclick={() => act(() => home && linkChannels(home.id, [], [...home.links]))}
      >
        {tr.channelMenu.unlinkAll}
      </button>
      <hr />
      <button role="menuitem" onclick={() => act(copyUrl)}>{tr.channelMenu.copyUrl}</button>
      <button role="menuitem" disabled={!has(cp, Perm.Write, Perm.TextMessage)} onclick={() => act(askChannelMessage)}>
        {tr.channelMenu.sendMessage}
      </button>
      {#if channel.id !== 0}
        <hr />
        {@render check(tr.channelMenu.hideWhenFiltering, channelFilter.modes.get(channel.id) === 'hide', true, () =>
          channelFilter.setMode(channel.id, 'hide'),
        )}
        {@render check(tr.channelMenu.pinWhenFiltering, channelFilter.modes.get(channel.id) === 'pin', true, () =>
          channelFilter.setMode(channel.id, 'pin'),
        )}
      {/if}
    {/if}
  </div>
{/if}

<style>
  .menu {
    position: fixed;
    max-height: calc(100vh - 16px);
    overflow-y: auto;
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
  hr {
    border: 0;
    border-top: 1px solid var(--border);
    margin: 0.2rem 0;
    width: 100%;
  }
  button:disabled {
    opacity: 0.45;
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
