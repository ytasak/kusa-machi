<script>
  import styles from './AppHeader.module.css';
  import Icon from './Icon.svelte';
  import { session, remainingMsFrom } from '../lib/session.svelte.js';
  import { ticker } from '../lib/ticker.svelte.js';
  import { countdown } from '../lib/format.js';

  // カウントダウンは常時表示なので、特定の画面ではなくヘッダーに置く。
  // どのタブを開いていてもその日の残り時間が見える。
  const remaining = $derived(countdown(remainingMsFrom(ticker.now)));
</script>

<header class={styles.header}>
  <span class={styles.countdown}>
    <Icon name="clock" size={15} />
    残り <span class={styles.time}>{remaining}</span>
  </span>

  <span class={styles.likes} aria-label="残りLike">
    <Icon name="heart" size={13} filled />
    {session.remainingLikes} / 10
  </span>
</header>
