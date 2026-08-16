<script>
  import styles from './Avatar.module.css';

  // The default picture is a plain silhouette, the same for everyone, exactly
  // like a matching app before the user uploads anything. The only per-persona
  // touch is a soft background tint derived from the persona id, so cards stay
  // distinguishable in a list while nobody has a picture yet.
  //
  // `photoUrl` is the slot a future uploaded picture drops into.
  let { persona, size = 56, shape = 'circle' } = $props();

  const HUES = [352, 12, 28, 336, 320, 4];

  function tint(id) {
    let sum = 0;
    for (const ch of id ?? '') sum = (sum + ch.charCodeAt(0)) % 4096;
    const hue = HUES[sum % HUES.length];
    return `linear-gradient(135deg, hsl(${hue} 78% 94%), hsl(${hue} 70% 88%))`;
  }

  const background = $derived(tint(persona?.id));
</script>

<div
  class="{styles.avatar} {shape === 'square' ? styles.square : ''}"
  style="width: {size}px; height: {size}px; --tint: {background}"
>
  {#if persona?.photo_url}
    <img class={styles.photo} src={persona.photo_url} alt="" />
  {:else}
    <svg
      class={styles.placeholder}
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
    >
      <circle cx="12" cy="8.5" r="4" />
      <path d="M12 14c-4.2 0-7.5 2.9-7.5 6.5h15C19.5 16.9 16.2 14 12 14z" />
    </svg>
  {/if}
</div>
