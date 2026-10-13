<script lang="ts">
  import { t } from '../i18n/index.svelte'
  import LangSwitch from './LangSwitch.svelte'
  import { accessPassword, fetchAccessRequired } from '../lib/accessPassword'
  import { connect } from '../lib/client'
  import { getSetting, setSetting } from '../lib/settings'
  import { session } from '../lib/stores/session.svelte'

  let username = $state(getSetting('username', ''))
  let password = $state('')
  const savedAccess = accessPassword.saved()
  let access = $state(savedAccess)
  let rememberAccess = $state(savedAccess !== '')
  const busy = $derived(session.status === 'connecting' || session.status === 'reconnecting')

  // Ask the gateway whether it wants its web access password; the field only appears then.
  $effect(() => {
    void fetchAccessRequired().then((required) => {
      if (required) session.accessRequired = true
    })
  })

  function submit(e: SubmitEvent) {
    e.preventDefault()
    const name = username.trim()
    if (!name) return
    setSetting('username', name)
    if (session.accessRequired) {
      if (rememberAccess && access) accessPassword.save(access)
      else accessPassword.forget()
    }
    connect(name, password, session.accessRequired ? access : '')
  }
</script>

<div class="wrap">
  <div class="lang-corner"><LangSwitch /></div>
  <form class="card" onsubmit={submit}>
    <h1>{t.appName}</h1>
    <p class="sub">{t.login.title}</p>

    {#if session.accessRequired}
      <label>
        <span>{t.login.accessPassword}</span>
        <input type="password" bind:value={access} autocomplete="off" required />
      </label>
      <label class="remember">
        <input type="checkbox" bind:checked={rememberAccess} />
        <span>{t.login.accessPasswordRemember}</span>
      </label>
    {/if}

    <label>
      <span>{t.login.nickname}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input type="text" bind:value={username} maxlength="64" autocomplete="nickname" required autofocus />
    </label>

    <label>
      <span>{t.login.password} <small>({t.login.passwordHint})</small></span>
      <input type="password" bind:value={password} autocomplete="current-password" />
    </label>

    {#if session.error}
      <p class="error" role="alert">{session.error}</p>
    {:else if session.notice && busy}
      <p class="notice" role="status">{session.notice}</p>
    {/if}

    <button class="primary" type="submit" disabled={busy || !username.trim() || (session.accessRequired && !access)}>
      {busy ? t.login.connecting : t.login.connect}
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
  .lang-corner {
    position: absolute;
    top: calc(env(safe-area-inset-top, 0px) + 0.75rem);
    right: var(--gutter);
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
  .remember {
    flex-direction: row;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
    color: var(--text-dim);
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
