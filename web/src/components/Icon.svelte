<script>
  // Inline SVG only: the app must stay self-contained and same-Origin.
  let { name, size = 24, filled = false } = $props();
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill={filled ? 'currentColor' : 'none'}
  stroke="currentColor"
  stroke-width="1.8"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  {#if name === 'search'}
    <circle cx="11" cy="11" r="7" fill="none" />
    <line x1="16.5" y1="16.5" x2="21" y2="21" />
  {:else if name === 'heart'}
    <path
      d="M12 20.7l-1.5-1.4C5.4 14.6 2 11.5 2 7.7 2 4.9 4.2 2.7 7 2.7c1.6 0 3.1.7 4 1.9.9-1.2 2.4-1.9 4-1.9 2.8 0 5 2.2 5 5 0 3.8-3.4 6.9-8.5 11.6L12 20.7z"
    />
  {:else if name === 'sparkle'}
    <path d="M12 2.5l2.3 6.2 6.2 2.3-6.2 2.3L12 19.5l-2.3-6.2L3.5 11l6.2-2.3L12 2.5z" />
  {:else if name === 'person'}
    <circle cx="12" cy="8" r="4" fill="none" />
    <path d="M4.5 21c0-4.1 3.4-7.5 7.5-7.5s7.5 3.4 7.5 7.5" />
  {:else if name === 'close'}
    <line x1="6" y1="6" x2="18" y2="18" />
    <line x1="18" y1="6" x2="6" y2="18" />
  {:else if name === 'back'}
    <path d="M15 5l-7 7 7 7" />
  {:else if name === 'clock'}
    <circle cx="12" cy="12" r="9" fill="none" />
    <path d="M12 7v5.2l3.2 2" />
  {:else if name === 'chevron'}
    <path d="M9 5l7 7-7 7" />
  {/if}
</svg>
