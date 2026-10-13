<script lang="ts">
  // Minimal inline icon set (stroke icons, 24x24 viewBox).
  type Name = 'mic' | 'micOff' | 'headphones' | 'headphonesOff' | 'logout' | 'user' | 'shield' | 'send' | 'channel' | 'settings' | 'heart' | 'megaphone' | 'ear' | 'link' | 'filter'
  let { name, size = 18 }: { name: Name; size?: number } = $props()
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
  stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#if name === 'mic' || name === 'micOff'}
    <rect x="9" y="2" width="6" height="12" rx="3" />
    <path d="M5 10v1a7 7 0 0 0 14 0v-1M12 18v4" />
    {#if name === 'micOff'}<path d="M3 3l18 18" />{/if}
  {:else if name === 'headphones' || name === 'headphonesOff'}
    <path d="M3 18v-6a9 9 0 0 1 18 0v6" />
    <path d="M21 19a2 2 0 0 1-2 2h-1v-6h3zM3 19a2 2 0 0 0 2 2h1v-6H3z" />
    {#if name === 'headphonesOff'}<path d="M3 3l18 18" />{/if}
  {:else if name === 'logout'}
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9" />
  {:else if name === 'user'}
    <circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" />
  {:else if name === 'shield'}
    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="M9 12l2 2 4-4" />
  {:else if name === 'send'}
    <path d="M22 2L11 13M22 2l-7 20-4-9-9-4z" />
  {:else if name === 'settings'}
    <circle cx="12" cy="12" r="3" />
    <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
  {:else if name === 'heart'}
    <path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1-1.1a5.5 5.5 0 0 0-7.8 7.8l1 1.1L12 21l7.8-7.5 1-1.1a5.5 5.5 0 0 0 0-7.8z" />
  {:else if name === 'megaphone'}
    <path d="M3 11v2a1 1 0 0 0 1 1h3l6 4V6L7 10H4a1 1 0 0 0-1 1zM16 9a4 4 0 0 1 0 6M19 6a8 8 0 0 1 0 12" />
  {:else if name === 'ear'}
    <path d="M6 8.5a6 6 0 0 1 12 0c0 3.5-3 4.5-3 7.5a3.5 3.5 0 0 1-6.5 1.8" /><path d="M9 9a3 3 0 0 1 6 0c0 2-2 2.5-2 4" />
  {:else if name === 'link'}
    <path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7" /><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7" />
  {:else if name === 'filter'}
    <path d="M22 3H2l8 9.5V19l4 2v-8.5z" />
  {:else if name === 'channel'}
    <path d="M4 9h16M4 15h16M10 3L8 21M16 3l-2 18" />
  {/if}
</svg>
