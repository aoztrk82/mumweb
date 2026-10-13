<script lang="ts">
  import { tr } from '../i18n/tr'
  import { session } from '../lib/stores/session.svelte'

  const text = $derived(
    session.status === 'reconnecting'
      ? tr.status.reconnecting(session.attempt, session.retryIn)
      : session.status === 'connecting' || (session.status === 'connected' && !session.ready)
        ? tr.status.connecting
        : session.status === 'connected'
          ? tr.status.connected
          : tr.status.idle,
  )
  const ok = $derived(session.status === 'connected' && session.ready)
</script>

<div class="status" class:ok class:warn={!ok} role="status">
  <span class="dot"></span>
  <span>{text}</span>
  {#if session.notice && !ok}<span class="notice">— {session.notice}</span>{/if}
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
