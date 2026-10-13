<script lang="ts">
  import Chat from './components/Chat.svelte'
  import ChannelTree from './components/ChannelTree.svelte'
  import ContextMenu from './components/ContextMenu.svelte'
  import Controls from './components/Controls.svelte'
  import CreateChannel from './components/CreateChannel.svelte'
  import Login from './components/Login.svelte'
  import Settings from './components/Settings.svelte'
  import StatusBar from './components/StatusBar.svelte'
  import { tr } from './i18n/tr'
  import { session } from './lib/stores/session.svelte'
  import { closeMenu } from './lib/stores/ui.svelte'

  const root = $derived(session.channels.get(0))
  let tab = $state<'channels' | 'chat'>('channels')
  let settingsOpen = $state(false)
</script>

{#if !session.joined}
  <Login />
{:else}
  <div class="app">
    <StatusBar />
    {#if session.audioUnsupported}<div class="banner" role="alert">{tr.audio.unsupported}</div>{/if}
    <nav class="tabs" aria-label="Görünüm">
      <button class:active={tab === 'channels'} onclick={() => (tab = 'channels')}>{tr.channels.title}</button>
      <button class:active={tab === 'chat'} onclick={() => (tab = 'chat')}>{tr.chat.title}</button>
    </nav>
    <main class="panes" data-tab={tab}>
      <aside class="channels" aria-label={tr.channels.title} onscroll={closeMenu}>
        {#if root}
          <ul class="tree"><ChannelTree channel={root} /></ul>
        {/if}
      </aside>
      <section class="chat-pane" aria-label={tr.chat.title}>
        <Chat />
      </section>
    </main>
    <Controls onSettings={() => (settingsOpen = true)} />
  </div>
  <Settings bind:open={settingsOpen} />
  <ContextMenu />
  <CreateChannel />
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
