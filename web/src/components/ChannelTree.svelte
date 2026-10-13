<script lang="ts">
  import { tr } from '../i18n/tr'
  import { joinChannel } from '../lib/client'
  import type { ChannelInfo } from '../lib/protocol'
  import { channelFilter } from '../lib/stores/channelFilter.svelte'
  import { channelListeners, channelUsers, childChannels, session, subtreeHasUsers } from '../lib/stores/session.svelte'
  import { openMenu, ui } from '../lib/stores/ui.svelte'
  import { userPref } from '../lib/stores/userPrefs.svelte'
  import { volumePref } from '../lib/stores/volumes.svelte'
  import ChannelTree from './ChannelTree.svelte'
  import Icon from './Icon.svelte'

  let { channel, depth = 0 }: { channel: ChannelInfo; depth?: number } = $props()

  const users = $derived(channelUsers(channel.id))
  const listeners = $derived(channelListeners(channel.id))
  const children = $derived(
    childChannels(channel.id).filter((c) => {
      // Desktop "filter": hide channels without users unless pinned; "hide" mode hides even with users.
      if (!channelFilter.enabled) return true
      const mode = channelFilter.modes.get(c.id)
      if (mode === 'pin') return true
      const hasSelf = session.self && (session.self.channelId === c.id || isAncestor(c.id, session.self.channelId))
      if (mode === 'hide') return !!hasSelf
      return hasSelf || subtreeHasUsers(c.id)
    }),
  )

  function isAncestor(ancestor: number, id: number): boolean {
    let c = session.channels.get(id)
    while (c && c.parent !== null) {
      if (c.parent === ancestor) return true
      c = session.channels.get(c.parent)
    }
    return false
  }
  const isCurrent = $derived(session.self?.channelId === channel.id)

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter') joinChannel(channel.id)
  }

</script>

<li class="channel" style:--depth={depth}>
  <div
    class="row channel-row"
    class:current={isCurrent}
    class:menu-open={ui.menu?.kind === 'channel' && ui.menu.id === channel.id}
    role="button"
    tabindex="0"
    aria-haspopup="menu"
    title={tr.channels.joinHint}
    ondblclick={() => joinChannel(channel.id)}
    oncontextmenu={(e) => openMenu(e, 'channel', channel.id)}
    onkeydown={onKey}
  >
    <Icon name="channel" size={15} />
    <span class="name">{channel.name}</span>
    {#if channel.links.length}<span class="dim-icon" title={tr.channelMenu.linked}><Icon name="link" size={13} /></span>{/if}
    {#if users.length}<span class="count">{users.length}</span>{/if}
  </div>

  {#if users.length}
    <ul class="users">
      {#each users as u (u.session)}
        {@const isSelf = u.session === session.selfSession}
        {@const speaking = session.speaking.has(u.session)}
        {@const pref = volumePref(u.name)}
        {@const up = userPref(u)}
        {#snippet content()}
          <span class="avatar" class:speaking title={speaking ? tr.channels.speaking : undefined}><Icon name="user" size={14} /></span>
          <span class="name" title={up.nickname ? tr.channels.realName(u.name) : undefined}>
            {up.nickname || u.name}{#if isSelf}&nbsp;<span class="dim">{tr.channels.you}</span>{/if}
          </span>
          {#if !isSelf && (pref.muted || pref.volume !== 1)}
            <span class="dim small">{pref.muted ? '0%' : `${Math.round(pref.volume * 100)}%`}</span>
          {/if}
          <span class="flags">
            {#if up.friend}<span class="heart" title={tr.channels.friend}><Icon name="heart" size={14} /></span>{/if}
            {#if up.ignoreMessages}<span title={tr.channels.ignored}>✕</span>{/if}
            {#if u.prioritySpeaker}<span title={tr.channels.prioritySpeaker}><Icon name="megaphone" size={14} /></span>{/if}
            {#if u.registered}<span title={tr.channels.registered}><Icon name="shield" size={14} /></span>{/if}
            {#if u.mute || u.suppress}<span class="red" title={tr.channels.serverMuted}><Icon name="micOff" size={14} /></span>
            {:else if u.selfMute}<span title={tr.channels.muted}><Icon name="micOff" size={14} /></span>{/if}
            {#if u.deaf}<span class="red" title={tr.channels.serverDeafened}><Icon name="headphonesOff" size={14} /></span>
            {:else if u.selfDeaf}<span title={tr.channels.deafened}><Icon name="headphonesOff" size={14} /></span>{/if}
          </span>
        {/snippet}
        <li>
          <div
            class="row user"
            class:self={isSelf}
            class:menu-open={ui.menu?.kind === 'user' && ui.menu.id === u.session}
            role="button"
            tabindex="0"
            aria-haspopup="menu"
            title={isSelf ? undefined : tr.channels.userHint}
            oncontextmenu={(e) => openMenu(e, 'user', u.session)}
          >
            {@render content()}
          </div>
        </li>
      {/each}
    </ul>
  {/if}

  {#if listeners.length}
    <ul class="users">
      {#each listeners as l (l.session)}
        <li class="row user listener" title={tr.channelMenu.listeners}>
          <span class="avatar"><Icon name="ear" size={14} /></span>
          <span class="name">{userPref(l).nickname || l.name}</span>
        </li>
      {/each}
    </ul>
  {/if}

  {#if children.length}
    <ul class="children">
      {#each children as c (c.id)}
        <ChannelTree channel={c} depth={depth + 1} />
      {/each}
    </ul>
  {/if}
</li>

<style>
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.3rem 0.5rem;
    padding-left: calc(0.5rem + var(--depth) * 1rem);
    border-radius: 6px;
    min-height: 2rem;
  }
  .channel-row {
    cursor: pointer;
    user-select: none;
    color: var(--text-dim);
    font-weight: 600;
  }
  .channel-row:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .channel-row.current {
    background: var(--bg-active);
    color: var(--text);
  }
  .user {
    padding-left: calc(1.6rem + var(--depth) * 1rem);
  }
  .row {
    -webkit-touch-callout: none; /* long press opens our menu, not the text selection UI */
  }
  .user {
    user-select: none;
  }
  .user:hover,
  .menu-open {
    background: var(--bg-hover);
  }
  .user.self .name {
    font-weight: 600;
  }
  .avatar.speaking {
    color: var(--speaking);
    box-shadow: 0 0 0 2px var(--speaking);
  }
  .small {
    font-size: 0.75rem;
  }
  .avatar {
    display: inline-grid;
    place-items: center;
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background: var(--bg-active);
    color: var(--text-dim);
    flex: none;
    box-shadow: 0 0 0 2px transparent;
    transition: box-shadow 0.1s;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .count {
    font-size: 0.8rem;
    font-weight: 400;
  }
  .dim {
    color: var(--text-dim);
    font-weight: 400;
  }
  .flags {
    display: inline-flex;
    gap: 0.25rem;
    color: var(--text-dim);
  }
  .red {
    color: var(--danger);
  }
  .listener {
    color: var(--text-dim);
    font-style: italic;
  }
  .dim-icon {
    color: var(--text-dim);
    display: inline-flex;
  }
  .heart {
    color: #e5484d;
  }
</style>
