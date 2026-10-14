<script lang="ts">
  import { LANGS, language, t } from '../i18n/index.svelte'
</script>

<div class="lang" role="group" aria-label={t.lang.label}>
  {#each LANGS as l (l)}
    <button
      type="button"
      class:active={language.current === l}
      aria-pressed={language.current === l}
      title={t.lang[l]}
      lang={l}
      onclick={() => language.set(l)}
    >
      {l.toUpperCase()}
    </button>
  {/each}
</div>

<style>
  .lang {
    display: inline-flex;
    flex-wrap: wrap;
    max-width: 100%;
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }
  button {
    padding: 0.25rem 0.45rem;
    border: 0;
    border-radius: 0;
    background: transparent;
    color: var(--text-dim);
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.03em;
    cursor: pointer;
  }
  button.active {
    background: var(--accent);
    color: var(--accent-text);
  }
</style>
