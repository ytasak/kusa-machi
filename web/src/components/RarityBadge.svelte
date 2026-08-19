<script>
  import styles from './RarityBadge.module.css';

  // Persona のレア度を示すバッジ。
  //
  // どのレア度かを決めるのは lib/gacha.js の personaRank だけで、
  // ここは受け取った結果を塗るだけ。一覧では静止したバッジ、探すの
  // カード切替時だけ reveal を立てて短い強調を鳴らす。
  let { rank, size = 'sm', reveal = false } = $props();
</script>

<span
  class="{styles.badge} {size === 'lg' ? styles.lg : ''} {reveal ? styles.reveal : ''}"
  data-rank={rank.label}
>
  <span class={styles.srOnly}>レア度</span>{rank.label}
</span>
