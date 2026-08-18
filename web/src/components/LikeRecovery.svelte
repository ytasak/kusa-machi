<script>
  // 時間回復の1行。ホームと探索の両方に置くので、条件はここに閉じ込める。
  //
  // 出すのは次の3つのうち1つだけ:
  //   - 回復した直後の通知（数秒で静かに消える）
  //   - 所持上限に達している「満タン」
  //   - 次の回復までのカウントダウン
  // どれにも当てはまらない状態、つまりまだ Like を1つも使っておらず
  // タイマーが動いていない場合は、何も出さない。
  import styles from './LikeRecovery.module.css';
  import Icon from './Icon.svelte';
  import { session, nextRecoveryMsFrom, clearRecoveryNotice } from '../lib/session.svelte.js';
  import { ticker } from '../lib/ticker.svelte.js';
  import { countdown } from '../lib/format.js';

  // 通知を残す時間。派手な演出は要らないので、黙って引っ込める。
  const NOTICE_MS = 4000;

  const untilNext = $derived(nextRecoveryMsFrom(ticker.now));
  const full = $derived(session.remainingLikes >= session.likeCapacity);

  // 出した通知は自分で片付ける。画面を離れても残らないよう、後始末も返す。
  $effect(() => {
    if (session.likesRecovered <= 0) return;
    const timer = setTimeout(clearRecoveryNotice, NOTICE_MS);
    return () => clearTimeout(timer);
  });
</script>

{#if session.likesRecovered > 0}
  <p class={styles.notice}>
    <Icon name="heart" size={13} filled />
    Likeが{session.likesRecovered}回復しました
  </p>
{:else if full}
  <p class={styles.line}>Like満タン</p>
{:else if untilNext !== null}
  <p class={styles.line}>
    <Icon name="clock" size={13} />
    次のLikeまで <span class={styles.time}>{countdown(untilNext)}</span>
  </p>
{/if}
