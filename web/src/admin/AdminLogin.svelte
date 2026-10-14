<script lang="ts">
  import { t } from '../i18n/index.svelte'
  import LangSwitch from '../components/LangSwitch.svelte'
  import PasswordField from '../components/PasswordField.svelte'
  import { getSetting, setSetting } from '../lib/settings'
  import { connect } from './client'
  import { admin } from './state.svelte'

  let username = $state(getSetting('adminUsername', 'SuperUser'))
  let password = $state('')
  let adminPassword = $state('')
  const busy = $derived(admin.status === 'connecting' || admin.status === 'reconnecting')

  function submit(e: SubmitEvent) {
    e.preventDefault()
    const name = username.trim()
    if (!name) return
    setSetting('adminUsername', name)
    connect(name, password, adminPassword)
  }
</script>

<div class="wrap">
  <div class="lang-corner"><LangSwitch /></div>
  <form class="card" onsubmit={submit}>
    <h1>{t.admin.title}</h1>
    <p class="hint">{t.admin.notAdminHint}</p>

    <label>
      <span>{t.admin.login.adminPassword}</span>
      <PasswordField bind:value={adminPassword} autocomplete="off" required autofocus />
    </label>
    <label>
      <span>{t.admin.login.username}</span>
      <input type="text" bind:value={username} maxlength="64" autocomplete="off" required />
    </label>
    <label>
      <span>{t.admin.login.password} <small>({t.admin.login.passwordHint})</small></span>
      <PasswordField bind:value={password} autocomplete="off" />
    </label>

    {#if admin.error}<p class="error" role="alert">{admin.error}</p>{/if}

    <button class="primary" type="submit" disabled={busy || !username.trim() || !adminPassword}>
      {busy ? t.admin.login.connecting : t.admin.login.connect}
    </button>
  </form>
</div>

<style>
  .wrap {
    min-height: 100dvh;
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
    width: min(100%, 400px);
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
    font-size: 1.4rem;
  }
  .hint {
    margin: 0;
    font-size: 0.85rem;
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
  .error {
    margin: 0;
    font-size: 0.9rem;
    color: var(--danger);
  }
  button {
    padding: 0.65rem;
  }
</style>
