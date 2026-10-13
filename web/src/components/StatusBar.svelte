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
  .spacer {
    flex: 1;
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
