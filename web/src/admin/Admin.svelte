<script lang="ts">
  import LangSwitch from '../components/LangSwitch.svelte'
  import { t } from '../i18n/index.svelte'
  import AdminLogin from './AdminLogin.svelte'
  import BansTab from './BansTab.svelte'
  import ChannelsTab from './ChannelsTab.svelte'
  import OnlineTab from './OnlineTab.svelte'
  import RegisteredTab from './RegisteredTab.svelte'
  import { disconnect } from './client'
  import { admin } from './state.svelte'

  type Tab = 'online' | 'channels' | 'bans' | 'registered'
  const TABS: Tab[] = ['online', 'channels', 'bans', 'registered']
  let tab = $state<Tab>('online')
</script>

{#if !admin.joined}
  <AdminLogin />
{:else}
  <div class="shell">
    <header>
      <h1>{t.admin.title}</h1>
      <span class="muted who">{t.admin.connectedAs(admin.self?.name ?? admin.username)}</span>
      <span class="grow"></span>
      <LangSwitch />
      <button type="button" onclick={disconnect}>{t.admin.logout}</button>
    </header>

    {#if admin.status === 'reconnecting'}
      <div class="banner" role="status">{t.admin.reconnecting(admin.retryIn)}</div>
    {/if}

    <nav class="tabs" aria-label={t.admin.title}>
      {#each TABS as id (id)}
        <button type="button" class:active={tab === id} onclick={() => (tab = id)}>{t.admin.tabs[id]}</button>
      {/each}
    </nav>

    <main>
      {#if tab === 'online'}<OnlineTab />{/if}
      {#if tab === 'channels'}<ChannelsTab />{/if}
      {#if tab === 'bans'}<BansTab />{/if}
      {#if tab === 'registered'}<RegisteredTab />{/if}
    </main>

    <div class="toasts" aria-live="polite">
      {#each admin.toasts as toast (toast.id)}
        <button type="button" class="toast" class:error={toast.kind === 'error'} onclick={() => admin.dismiss(toast.id)}>{toast.text}</button>
      {/each}
    </div>
  </div>
{/if}

<style>
  .shell {
    max-width: 1100px;
    margin: 0 auto;
    padding: 0 var(--gutter) 3rem;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem;
    padding: 0.9rem 0;
  }
  h1 {
    margin: 0;
    font-size: 1.25rem;
  }
  .who {
    font-size: 0.9rem;
  }
  .grow {
    flex: 1;
  }
  .banner {
    padding: 0.5rem 0.75rem;
    margin-bottom: 0.75rem;
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--warn) 18%, var(--bg));
    font-size: 0.9rem;
  }
  .tabs {
    display: flex;
    flex-wrap: wrap;
    border-bottom: 1px solid var(--border);
    margin-bottom: 1rem;
  }
  .tabs button {
    border: 0;
    border-radius: 0;
    background: transparent;
    padding: 0.6rem 1rem;
    border-bottom: 2px solid transparent;
  }
  .tabs button.active {
    border-bottom-color: var(--accent);
    font-weight: 600;
  }
  .toasts {
    position: fixed;
    right: var(--gutter);
    bottom: var(--gutter);
    left: var(--gutter);
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 0.4rem;
    pointer-events: none;
  }
  .toast {
    pointer-events: auto;
    max-width: min(100%, 420px);
    text-align: left;
    background: var(--bg-elev);
    box-shadow: 0 4px 16px rgb(0 0 0 / 0.35);
  }
  .toast.error {
    border-color: var(--danger);
  }
</style>
