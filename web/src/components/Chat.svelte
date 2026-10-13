<script lang="ts">
  import { tick } from 'svelte'
  import { tr } from '../i18n/tr'
  import { sendText } from '../lib/client'
  import { session } from '../lib/stores/session.svelte'
  import Icon from './Icon.svelte'

  let draft = $state('')
  let log: HTMLElement | undefined = $state()
  let stickToBottom = true

  const timeFmt = new Intl.DateTimeFormat('tr-TR', { hour: '2-digit', minute: '2-digit' })

  function onScroll() {
    if (!log) return
    stickToBottom = log.scrollHeight - log.scrollTop - log.clientHeight < 40
  }

  $effect(() => {
    void session.chat.length
    if (stickToBottom) tick().then(() => log?.scrollTo({ top: log.scrollHeight }))
  })

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (sendText(draft)) {
      draft = ''
      stickToBottom = true
    }
  }

  function onKey(e: KeyboardEvent) {
    // Enter sends, Shift+Enter inserts a newline.
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault()
      ;(e.currentTarget as HTMLTextAreaElement).form?.requestSubmit()
    }
  }
</script>

<section class="chat">
  <div class="log" bind:this={log} onscroll={onScroll} aria-live="polite">
    {#each session.chat as m (m.id)}
      <div class="msg {m.kind}" class:own={m.fromSelf}>
        <time datetime={m.time.toISOString()}>{timeFmt.format(m.time)}</time>
        {#if m.kind === 'text'}
          <span class="from">{m.fromSelf ? (m.target ? tr.chat.toUser(m.target) : tr.chat.you) : m.from}{#if m.private}&nbsp;<em>({tr.chat.private})</em>{/if}:</span>
          <!-- Sanitized with DOMPurify in sanitize.ts -->
          <span class="body">{@html m.body}</span>
        {:else}
          <span class="body">{m.body}</span>
        {/if}
      </div>
    {/each}
  </div>

  <form class="compose" onsubmit={submit}>
    <label class="visually-hidden" for="chat-input">{tr.chat.title}</label>
    <textarea
      id="chat-input"
      rows="1"
      bind:value={draft}
      onkeydown={onKey}
      placeholder={tr.chat.placeholder(session.currentChannel?.name ?? '')}
      disabled={!session.ready}
    ></textarea>
    <button class="primary" type="submit" disabled={!session.ready || !draft.trim()} title={tr.chat.send}>
      <Icon name="send" />
      <span class="visually-hidden">{tr.chat.send}</span>
    </button>
  </form>
</section>

<style>
  .chat {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
  }
  .log {
    flex: 1;
    overflow-y: auto;
    padding: 0.75rem var(--gutter);
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .msg {
    overflow-wrap: anywhere;
  }
  time {
    color: var(--text-dim);
    font-size: 0.8rem;
    margin-right: 0.4rem;
    font-variant-numeric: tabular-nums;
  }
  .from {
    font-weight: 600;
    margin-right: 0.3rem;
  }
  .own .from {
    color: var(--accent);
  }
  .system .body {
    color: var(--text-dim);
    font-style: italic;
  }
  .error .body {
    color: var(--danger);
  }
  .body :global(img) {
    max-width: min(100%, 320px);
    max-height: 240px;
    display: block;
    margin-top: 0.25rem;
    border-radius: 6px;
  }
  .body :global(p) {
    margin: 0;
  }
  .body :global(a) {
    color: var(--accent);
  }
  .compose {
    display: flex;
    gap: 0.5rem;
    padding: 0.6rem var(--gutter);
    border-top: 1px solid var(--border);
  }
  textarea {
    resize: none;
    min-height: 2.4rem;
    max-height: 8rem;
    field-sizing: content;
  }
  .compose button {
    display: inline-grid;
    place-items: center;
    padding: 0 0.8rem;
  }
</style>
