<script lang="ts">
  import { tr } from '../i18n/tr'
  import { joinChannel } from '../lib/client'
  import type { ChannelInfo } from '../lib/protocol'
  import { channelUsers, childChannels, session } from '../lib/stores/session.svelte'
  import { openMenu, ui } from '../lib/stores/ui.svelte'
  import { volumePref } from '../lib/stores/volumes.svelte'
  import ChannelTree from './ChannelTree.svelte'
  import Icon from './Icon.svelte'

  let { channel, depth = 0 }: { channel: ChannelInfo; depth?: number } = $props()

  const users = $derived(channelUsers(channel.id))
  const children = $derived(childChannels(channel.id))
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
    {#if users.length}<span class="count">{users.length}</span>{/if}
  </div>

  {#if users.length}
    <ul class="users">
      {#each users as u (u.session)}
        {@const isSelf = u.session === session.selfSession}
        {@const speaking = session.speaking.has(u.session)}
        {@const pref = volumePref(u.name)}
        {#snippet content()}
          <span class="avatar" class:speaking title={speaking ? tr.channels.speaking : undefined}><Icon name="user" size={14} /></span>
          <span class="name">{u.name}{#if isSelf}&nbsp;<span class="dim">{tr.channels.you}</span>{/if}</span>
          {#if !isSelf && (pref.muted || pref.volume !== 1)}
            <span class="dim small">{pref.muted ? '0%' : `${Math.round(pref.volume * 100)}%`}</span>
          {/if}
          <span class="flags">
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
</style>
