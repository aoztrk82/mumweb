<script lang="ts">
  import { tr } from '../i18n/tr'
  import { connect } from '../lib/client'
  import { getSetting, setSetting } from '../lib/settings'
  import { session } from '../lib/stores/session.svelte'

  let username = $state(getSetting('username', ''))
  let password = $state('')
  const busy = $derived(session.status === 'connecting' || session.status === 'reconnecting')

  function submit(e: SubmitEvent) {
    e.preventDefault()
    const name = username.trim()
    if (!name) return
    setSetting('username', name)
    connect(name, password)
  }
</script>

<div class="wrap">
  <form class="card" onsubmit={submit}>
    <h1>{tr.appName}</h1>
    <p class="sub">{tr.login.title}</p>

    <label>
      <span>{tr.login.nickname}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input type="text" bind:value={username} maxlength="64" autocomplete="nickname" required autofocus />
    </label>

    <label>
      <span>{tr.login.password} <small>({tr.login.passwordHint})</small></span>
      <input type="password" bind:value={password} autocomplete="current-password" />
    </label>

    {#if session.error}
      <p class="error" role="alert">{session.error}</p>
    {:else if session.notice && busy}
      <p class="notice" role="status">{session.notice}</p>
    {/if}

    <button class="primary" type="submit" disabled={busy || !username.trim()}>
      {busy ? tr.login.connecting : tr.login.connect}
    </button>
  </form>
</div>

<style>
  .wrap {
    min-height: 100%;
    display: grid;
    place-items: center;
    padding: var(--gutter);
  }
  .card {
    width: min(100%, 360px);
    background: var(--bg-elev);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
  }
  .sub {
    margin: -0.5rem 0 0.25rem;
    color: var(--text-dim);
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  small {
    color: var(--text-dim);
  }
  .error,
  .notice {
    margin: 0;
    font-size: 0.9rem;
  }
  .error {
    color: var(--danger);
  }
  .notice {
    color: var(--warn);
  }
  button {
    padding: 0.65rem;
  }
</style>
