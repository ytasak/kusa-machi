<script>
  import styles from './Avatar.module.css';

  // 既定の画像は全員共通のシルエット。写真を上げる前のマッチングアプリと同じ。
  // Persona ごとに変えるのは persona id から導いた淡い背景色だけで、
  // 誰も写真を持っていない状態でも一覧で見分けがつくようにしている。
  //
  // 写真がアップロードされると persona.photo_url がこのスロットに入る。
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
