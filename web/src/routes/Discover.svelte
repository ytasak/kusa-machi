<script>
  import { onMount } from 'svelte';
  import styles from './Discover.module.css';
  import ui from '../components/ui.module.css';
  import Icon from '../components/Icon.svelte';
  import PersonaCard from '../components/PersonaCard.svelte';
  import MatchAnimation from '../components/MatchAnimation.svelte';
  import { api, ApiError } from '../lib/api.js';
  import { session, withDayGuard, applyLikeState } from '../lib/session.svelte.js';
  import { discover, currentCard, consumeCurrent, ensureCards } from '../lib/discover.svelte.js';
  import { go, SCREENS } from '../lib/nav.svelte.js';
  import { errorMessage } from '../lib/errors.js';
  import { vibrate, HAPTICS } from '../lib/haptics.js';

  // 送り出したカードが画面外へ抜けるまでの時間。CSS の flyLike / flyPass と揃える。
  const EXIT_MS = 460;

  let message = $state(null);
  /** 送り出し中のカード: { persona, kind, seq } */
  let exiting = $state(null);
  let matchedPersona = $state(null);
  /** 直近の Match で回復した Like 数。演出の中に出す。 */
  let matchedLikesGained = $state(0);

  const card = $derived(currentCard());
  const outOfLikes = $derived(session.remainingLikes <= 0);

  onMount(refill);

  /**
   * カードを補充する。探索の応答には残数と次回回復も入っているので、
   * この往復が時間回復を画面へ反映する機会にもなる。
   */
  async function refill() {
    const res = await ensureCards();
    if (res) applyLikeState(res);
  }

  // 「このカードはもう古い」という意味でしかないエラー。カードはすでに
  // 送り出しているので、ユーザーに見せるべきことは何も無い。
  const STALE_CODES = new Set(['AlreadyLiked', 'TargetPersonaUnavailable', 'PassLimitReached']);

  let exitTimer = null;
  let throwSeq = 0;

  /**
   * 判定したカードを次のカードの上に重ねてから飛ばす。
   *
   * seq を鍵にして毎回別の要素として描画するので、連打しても
   * アニメーションが最初から再生され、押した回数ぶん手応えが返る。
   */
  function throwCard(persona, kind) {
    exiting = { persona, kind, seq: ++throwSeq };
    clearTimeout(exitTimer);
    exitTimer = setTimeout(() => {
      exiting = null;
    }, EXIT_MS);
  }

  /**
   * Like と Pass は応答を待たずに画面を進める。
   *
   * サーバは常に正であり、成功時のレスポンスで残数を確定させる。往復のあいだ
   * 指を止めさせないためにこうしている。1往復ぶん（本番で約80ms）の待ちが
   * 体感から消える。
   */
  async function onLike() {
    const target = card;
    if (!target || outOfLikes) return;

    message = null;

    // 楽観更新。ここで残数を減らすので、連打しても上限を超えて送れない。
    session.remainingLikes -= 1;
    consumeCurrent();
    throwCard(target, 'like');
    vibrate(HAPTICS.like);
    refill();

    try {
      const res = await withDayGuard(() => api.post('/api/likes', { persona_id: target.id }));

      // サーバの値で確定させ、楽観更新のズレをここで吸収する。
      applyLikeState(res);

      if (res.matched) {
        session.matchCount += 1;
        matchedPersona = res.target_persona;
        matchedLikesGained = res.likes_gained;
        vibrate(HAPTICS.match);
      }
    } catch (e) {
      revertLike(e);
    }
  }

  // 失敗した Like は消費されていないので残数を戻す。ズレても次の成功時に
  // サーバの値で上書きされる。
  function revertLike(e) {
    if (e instanceof ApiError && e.code === 'LikeLimitExceeded') {
      session.remainingLikes = 0;
    } else {
      session.remainingLikes = Math.min(session.likeCapacity, session.remainingLikes + 1);
    }

    if (!(e instanceof ApiError) || !STALE_CODES.has(e.code)) {
      message = errorMessage(e);
    }
  }

  async function onPass() {
    const target = card;
    if (!target) return;

    message = null;

    // Pass は消費する予算が無いので、送り出したら結果を待つ必要がない。
    // 3回目でサーバが当日除外にするため、ローカルのクールダウンは常に付けてよい。
    consumeCurrent({ cooldownId: target.id });
    throwCard(target, 'pass');
    vibrate(HAPTICS.pass);
    refill();

    try {
      await withDayGuard(() => api.post('/api/passes', { persona_id: target.id }));
    } catch (e) {
      if (!(e instanceof ApiError) || !STALE_CODES.has(e.code)) {
        message = errorMessage(e);
      }
    }
  }
</script>

<section class={styles.screen}>
  <div class={styles.topRow}>
    <h1 class={ui.title}>探す</h1>
    <span class={styles.receivedBadge}>
      <Icon name="heart" size={13} filled />
      Likeされた {session.receivedLikeCount}
    </span>
  </div>

  {#if message}
    <p class={ui.error}>{message}</p>
  {/if}

  {#if card || exiting}
    <div class={styles.stage}>
      <div class={styles.cardBox}>
        {#if card}
          <!-- key を付けることで、カードが変わるたびに飛び出す動きが走る。 -->
          {#key card.id}
            <div class={styles.cardLayer}>
              <PersonaCard persona={card} variant="hero" />
            </div>
          {/key}
        {/if}

        {#if exiting}
          {#key exiting.seq}
            <div
              class="{styles.exitLayer} {exiting.kind === 'like' ? styles.exitLike : styles.exitPass}"
            >
              <PersonaCard persona={exiting.persona} variant="hero" />
              <span
                class="{styles.tint} {exiting.kind === 'like' ? styles.tintLike : styles.tintPass}"
              ></span>
              <span
                class="{styles.stamp} {exiting.kind === 'like' ? styles.stampLike : styles.stampPass}"
              >
                {exiting.kind === 'like' ? 'LIKE' : 'PASS'}
              </span>
            </div>
          {/key}
        {/if}
      </div>
    </div>

    <div class={ui.actions}>
      <button class="{ui.circle} {ui.circlePass}" onclick={onPass} aria-label="パス">
        <Icon name="close" size={26} />
      </button>
      <button
        class="{ui.circle} {ui.circleLike}"
        onclick={onLike}
        disabled={outOfLikes}
        aria-label="Like"
      >
        <Icon name="heart" size={30} filled />
      </button>
    </div>

    <!-- Like が尽きた瞬間がいちばん「どうすれば増えるのか」を知りたいところ。
         まだ取れる報酬があればそれを出し、無ければ Match だけを案内する。 -->
    {#if outOfLikes}
      {#if session.profileRewardAvailable}
        <button class={styles.recovery} onclick={() => go(SCREENS.profile)}>
          Likeを使い切りました。プロフィールを完成させると<strong>+1</strong>戻ります
        </button>
      {:else}
        <p class={styles.recoveryNote}>
          Likeを使い切りました。Matchが決まると<strong>+2</strong>戻ります（1日2回まで）
        </p>
      {/if}
    {/if}
  {:else if discover.loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if discover.error}
    <p class={ui.error}>{discover.error}</p>
  {:else}
    <p class={ui.empty}>いま出会える人がいません。<br />しばらくしてからまた見てみてください。</p>
  {/if}
</section>

{#if matchedPersona}
  <MatchAnimation
    own={session.persona}
    counterpart={matchedPersona}
    likesGained={matchedLikesGained}
    onclose={() => (matchedPersona = null)}
  />
{/if}
