<script lang="ts">
  import { t } from '../i18n/index.svelte'
  import LangSwitch from './LangSwitch.svelte'
  import { session } from '../lib/stores/session.svelte'

  const text = $derived(
    session.status === 'reconnecting'
      ? t.status.reconnecting(session.attempt, session.retryIn)
      : session.status === 'connecting' || (session.status === 'connected' && !session.ready)
        ? t.status.connecting
        : session.status === 'connected'
          ? t.status.connected
          : t.status.idle,
  )
  const ok = $derived(session.status === 'connected' && session.ready)
</script>

<div class="status" class:ok class:warn={!ok} role="status">
  <span class="dot"></span>
  <span>{text}</span>
  {#if session.notice && !ok}<span class="notice">— {session.notice}</span>{/if}
  {#if ok && session.serverName}
    <span class="server-name" title={session.serverDescription || undefined}>· {session.serverName}</span>
  {/if}
  {#if ok && session.serverDescription}
    <span class="server-desc">{session.serverDescription}</span>
  {/if}
  <span class="spacer"></span>
  <LangSwitch />
</div>

<style>
  .status {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.35rem var(--gutter);
    font-size: 0.85rem;
    color: var(--text-dim);
    border-bottom: 1px solid var(--border);
    background: var(--bg-elev);
  }
  .server-name {
    color: var(--text);
    font-weight: 600;
    flex: none;
  }
  .server-desc {
    color: var(--text-dim);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .spacer {
    flex: 1;
    min-width: 0.5rem;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--warn);
    flex: none;
  }
  .ok .dot {
    background: var(--speaking);
  }
  .warn {
    color: var(--warn);
  }
</style>
