<script>
  import { onMount } from 'svelte';
  import styles from './Discover.module.css';
  import ui from '../components/ui.module.css';
  import Icon from '../components/Icon.svelte';
  import PersonaCard from '../components/PersonaCard.svelte';
  import MatchAnimation from '../components/MatchAnimation.svelte';
  import { api, ApiError } from '../lib/api.js';
  import { session, withDayGuard, LIKE_BUDGET } from '../lib/session.svelte.js';
  import { discover, currentCard, consumeCurrent, ensureCards } from '../lib/discover.svelte.js';
  import { errorMessage } from '../lib/errors.js';

  // スタンプ演出の長さ。CSS の flashBurst / stampIn と揃える。
  const FLASH_MS = 400;

  let message = $state(null);
  /** 'like' | 'pass' | null */
  let flash = $state(null);
  let matchedPersona = $state(null);

  const card = $derived(currentCard());
  const outOfLikes = $derived(session.remainingLikes <= 0);

  onMount(ensureCards);

  // 「このカードはもう古い」という意味でしかないエラー。カードはすでに
  // 送り出しているので、ユーザーに見せるべきことは何も無い。
  const STALE_CODES = new Set(['AlreadyLiked', 'TargetPersonaUnavailable', 'PassLimitReached']);

  let flashTimer = null;

  function showFlash(kind) {
    // 一度消してから出し直す。連打してもアニメーションが再生され、
    // 押した回数ぶんだけ手応えが返る。
    flash = null;
    clearTimeout(flashTimer);
    requestAnimationFrame(() => {
      flash = kind;
      flashTimer = setTimeout(() => {
        flash = null;
      }, FLASH_MS);
    });
  }

  function clearFlash() {
    clearTimeout(flashTimer);
    flash = null;
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
    showFlash('like');
    ensureCards();

    try {
      const res = await withDayGuard(() => api.post('/api/likes', { persona_id: target.id }));

      // サーバの値で確定させ、楽観更新のズレをここで吸収する。
      session.remainingLikes = res.remaining_likes;

      if (res.matched) {
        session.matchCount += 1;
        // Match のほうが強い演出なので、LIKE のスタンプは引っ込める。
        clearFlash();
        matchedPersona = res.target_persona;
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
      session.remainingLikes = Math.min(LIKE_BUDGET, session.remainingLikes + 1);
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
    showFlash('pass');
    ensureCards();

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

  {#if card}
    <div class={styles.stage}>
      <!-- key を付けることで、カードが変わるたびに飛び出すアニメーションが走る。 -->
      {#key card.id}
        <div class={styles.cardLayer}>
          <PersonaCard persona={card} variant="hero" />
        </div>
      {/key}

      {#if flash}
        <div class="{styles.flash} {flash === 'like' ? styles.flashLike : styles.flashPass}">
          <span class="{styles.stamp} {flash === 'pass' ? styles.stampPass : ''}">
            {flash === 'like' ? 'LIKE' : 'PASS'}
          </span>
        </div>
      {/if}
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
    onclose={() => (matchedPersona = null)}
  />
{/if}
