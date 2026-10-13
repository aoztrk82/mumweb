<script lang="ts">
  import Chat from './components/Chat.svelte'
  import ChannelTree from './components/ChannelTree.svelte'
  import ContextMenu from './components/ContextMenu.svelte'
  import Controls from './components/Controls.svelte'
  import CreateChannel from './components/CreateChannel.svelte'
  import Login from './components/Login.svelte'
  import Settings from './components/Settings.svelte'
  import UserDialogs from './components/UserDialogs.svelte'
  import StatusBar from './components/StatusBar.svelte'
  import { t } from './i18n/index.svelte'
  import { session } from './lib/stores/session.svelte'
  import { closeMenu } from './lib/stores/ui.svelte'
  import { channelFilter } from './lib/stores/channelFilter.svelte'
  import Icon from './components/Icon.svelte'

  const root = $derived(session.channels.get(0))
  let tab = $state<'channels' | 'chat'>('channels')
  let settingsOpen = $state(false)
</script>

{#if !session.joined}
  <Login />
{:else}
  <div class="app">
    <StatusBar />
    {#if session.audioUnsupported}<div class="banner" role="alert">{t.audio.unsupported}</div>{/if}
    <nav class="tabs" aria-label={t.nav}>
      <button class:active={tab === 'channels'} onclick={() => (tab = 'channels')}>{t.channels.title}</button>
      <button class:active={tab === 'chat'} onclick={() => (tab = 'chat')}>{t.chat.title}</button>
    </nav>
    <main class="panes" data-tab={tab}>
      <aside class="channels" aria-label={t.channels.title} onscroll={closeMenu}>
        <div class="tree-head">
          <span>{t.channels.title}</span>
          <button
            class="icon-btn"
            class:active={channelFilter.enabled}
            aria-pressed={channelFilter.enabled}
            title={t.channelMenu.filter}
            onclick={() => channelFilter.toggle()}
          >
            <Icon name="filter" size={15} />
          </button>
        </div>
        {#if root}
          <ul class="tree"><ChannelTree channel={root} /></ul>
        {/if}
      </aside>
      <section class="chat-pane" aria-label={t.chat.title}>
        <Chat />
      </section>
    </main>
    <Controls onSettings={() => (settingsOpen = true)} />
  </div>
  <Settings bind:open={settingsOpen} />
  <ContextMenu />
  <CreateChannel />
  <UserDialogs />
{/if}

<style>
  .app {
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  .panes {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(240px, 340px) 1fr;
  }
  .channels {
    overflow-y: auto;
    border-right: 1px solid var(--border);
    padding: 0.5rem;
  }
  .tree-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.1rem 0.5rem 0.4rem;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-dim);
  }
  .icon-btn {
    display: inline-grid;
    place-items: center;
    padding: 0.25rem;
    border: 1px solid transparent;
    background: transparent;
    color: var(--text-dim);
  }
  .icon-btn.active {
    color: var(--accent);
    border-color: var(--accent);
  }
  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .chat-pane {
    min-height: 0;
    min-width: 0;
  }
  .tabs {
    display: none;
  }
  .banner {
    padding: 0.5rem var(--gutter);
    background: color-mix(in srgb, var(--danger) 18%, var(--bg));
    color: var(--text);
    font-size: 0.9rem;
  }

  @media (max-width: 640px) {
    .tabs {
      display: flex;
      border-bottom: 1px solid var(--border);
    }
    .tabs button {
      flex: 1;
      border: 0;
      border-radius: 0;
      background: transparent;
      padding: 0.6rem;
      border-bottom: 2px solid transparent;
    }
    .tabs button.active {
      border-bottom-color: var(--accent);
      font-weight: 600;
    }
    .panes {
      grid-template-columns: 1fr;
    }
    .channels {
      border-right: 0;
    }
    .panes[data-tab='channels'] .chat-pane,
    .panes[data-tab='chat'] .channels {
      display: none;
    }
  }
</style>
