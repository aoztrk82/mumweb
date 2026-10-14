<script lang="ts">
  import { t } from '../i18n/index.svelte'

  interface Props {
    value: string
    id?: string
    autocomplete?: string
    required?: boolean
    autofocus?: boolean
    maxlength?: number
    disabled?: boolean
  }

  let { value = $bindable(''), id, autocomplete = 'off', required, autofocus, maxlength, disabled }: Props = $props()

  let revealed = $state(false)
  let copied = $state(false)
  let copiedTimer: ReturnType<typeof setTimeout> | undefined

  async function copy() {
    if (!value) return
    try {
      await navigator.clipboard.writeText(value)
      copied = true
      clearTimeout(copiedTimer)
      copiedTimer = setTimeout(() => (copied = false), 1500)
    } catch {
      // Clipboard API unavailable or permission denied; nothing more we can do.
    }
  }
</script>

<div class="password-field">
  <!-- svelte-ignore a11y_autofocus -->
  <!-- svelte-ignore a11y_autocomplete_valid -->
  <input {id} type={revealed ? 'text' : 'password'} bind:value autocomplete={autocomplete as 'off'} {required} {autofocus} {maxlength} {disabled} />
  <div class="icons">
    <button
      type="button"
      class="icon"
      title={revealed ? t.password.hide : t.password.show}
      aria-label={revealed ? t.password.hide : t.password.show}
      aria-pressed={revealed}
      onclick={() => (revealed = !revealed)}
    >
      {#if revealed}
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <path
            d="M3 3l18 18M10.6 10.7a2 2 0 002.8 2.8M9.3 5.5A10.5 10.5 0 0112 5c5 0 9 3.3 10.5 7-.6 1.4-1.5 2.7-2.6 3.8M6.1 6.2C4.1 7.5 2.6 9.5 1.5 12c1.5 3.7 5.5 7 10.5 7 1.3 0 2.5-.2 3.6-.6"
          />
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M1.5 12C3 8.3 7 5 12 5s9 3.3 10.5 7C21 15.7 17 19 12 19S3 15.7 1.5 12z" />
          <circle cx="12" cy="12" r="3" />
        </svg>
      {/if}
    </button>
    <button
      type="button"
      class="icon"
      title={copied ? t.password.copied : t.password.copy}
      aria-label={copied ? t.password.copied : t.password.copy}
      disabled={!value}
      onclick={copy}
    >
      {#if copied}
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M4 12l5 5L20 6" />
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="8" y="8" width="12" height="12" rx="2" />
          <path d="M16 8V5a1 1 0 00-1-1H5a1 1 0 00-1 1v10a1 1 0 001 1h3" />
        </svg>
      {/if}
    </button>
  </div>
</div>

<style>
  .password-field {
    position: relative;
  }
  .password-field input {
    padding-right: 4.3rem;
  }
  .icons {
    position: absolute;
    inset-block: 0;
    right: 0.3rem;
    display: flex;
    align-items: center;
    gap: 0.1rem;
  }
  .icon {
    border: 0;
    background: transparent;
    padding: 0.3rem;
    border-radius: var(--radius);
    color: var(--text-dim);
    display: inline-flex;
  }
  .icon:hover:not(:disabled) {
    color: var(--text);
    background: var(--bg-hover);
  }
  .icon:disabled {
    opacity: 0.4;
    cursor: default;
  }
</style>
