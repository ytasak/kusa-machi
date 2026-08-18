<script>
  import styles from './AppHeader.module.css';
  import Icon from './Icon.svelte';
  import { session, remainingMsFrom, nextRecoveryMsFrom } from '../lib/session.svelte.js';
  import { ticker } from '../lib/ticker.svelte.js';
  import { countdown, minutesLeft } from '../lib/format.js';

  // 1ポイント増やすたびの間隔。まとめて回復した日も、増えた数だけ数字が
  // 上がるのがそのまま見える。
  const STEP_MS = 380;

  // カウントダウンは常時表示なので、特定の画面ではなくヘッダーに置く。
  // どのタブを開いていてもその日の残り時間が見える。
  const remaining = $derived(countdown(remainingMsFrom(ticker.now)));

  // 次の回復までの残り時間。null なら何も出さない。null になる条件（まだ Like を
  // 使っていない・満タン・日をまたぐ回復）はサーバが判定済み。
  const untilNext = $derived(nextRecoveryMsFrom(ticker.now));

  // 表示用の残数。正はあくまでサーバの session.remainingLikes で、増えたときだけ
  // 1ポイントずつ追いつかせる。クライアントのタイマーが残数を増やすことはない。
  //
  // 初期値を回復ぶんだけ戻しておくのは、アプリを開いた時点で回復が反映済みの
  // ときにも増える様子を見せるため。「戻ってきたら増えていた」がこの機能の
  // 主役なので、その瞬間だけ演出が無いのはおかしい。
  const startAt = Math.max(0, session.remainingLikes - session.likesRecovered);

  let shown = $state(startAt);
  // アニメーションを再生し直すための鍵。Discover の送り出しと同じ作法で、
  // 増えるたびに別の要素として描画することで演出が最初から流れる。
  let step = $state(0);

  // mirror は shown のリアクティブでない写し。$effect の依存に入らないので、
  // 1ステップ進めるたびに effect が走り直すことがない。
  let mirror = startAt;
  let timer = null;

  $effect(() => {
    const target = session.remainingLikes;

    if (target > mirror) {
      timer = setInterval(() => {
        mirror += 1;
        shown = mirror;
        step += 1;
        if (mirror >= target) {
          clearInterval(timer);
          timer = null;
        }
      }, STEP_MS);
      return () => clearInterval(timer);
    }

    // 消費や日付替わりで減ったときは、待たずにその場で合わせる。
    mirror = target;
    shown = target;
  });
</script>

<header class={styles.header}>
  <span class={styles.countdown}>
    <Icon name="clock" size={15} />
    残り <span class={styles.time}>{remaining}</span>
  </span>

  <span class={styles.likes}>
    <Icon name="heart" size={13} filled />
    {#key step}
      <span class="{styles.count} {step > 0 ? styles.gained : ''}" aria-label="残りLike">
        {shown} / {session.likeCapacity}
      </span>
    {/key}

    <!-- 回復までの時間は残数のすぐ隣に置く。数字と「それが次にいつ増えるか」は
         同じことを言っているので、離して置くと結び付かない。 -->
    {#if untilNext !== null}
      <span class={styles.recovery}>
        <!-- 狭い画面ではラベルを引っ込めて、この時計だけで「あと何分か」を示す。
             ヘッダーは1行に収まらないと高さが崩れるため、文字を折り返させない。 -->
        <span class={styles.recoveryIcon}><Icon name="clock" size={11} /></span>
        <span class={styles.recoveryLabel}>Like回復まで</span>
        <span class={styles.time}>{minutesLeft(untilNext)}</span>
      </span>
    {/if}
  </span>
</header>
