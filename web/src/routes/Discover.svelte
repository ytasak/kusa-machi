<script>
  import { onMount } from 'svelte';
  import styles from './Discover.module.css';
  import ui from '../components/ui.module.css';
  import PersonaCard from '../components/PersonaCard.svelte';
  import MatchAnimation from '../components/MatchAnimation.svelte';
  import { api, ApiError } from '../lib/api.js';
  import { session, withDayGuard } from '../lib/session.svelte.js';
  import { discover, currentCard, consumeCurrent, dropFromQueue, ensureCards } from '../lib/discover.svelte.js';
  import { goHome } from '../lib/nav.svelte.js';

  const LIKE_FLASH_MS = 450;

  let busy = $state(false);
  let message = $state(null);
  let likeFlash = $state(false);
  let matchedPersona = $state(null);

  const card = $derived(currentCard());
  const outOfLikes = $derived(session.remainingLikes <= 0);

  onMount(ensureCards);

  // Errors that simply mean "this card is stale": drop it and move on.
  const STALE_CODES = new Set(['AlreadyLiked', 'TargetPersonaUnavailable', 'PassLimitReached']);

  function handleActionError(e, personaId) {
    if (!(e instanceof ApiError)) {
      message = e.message;
      return;
    }
    if (STALE_CODES.has(e.code)) {
      dropFromQueue(personaId);
      return;
    }
    if (e.code === 'LikeLimitExceeded') {
      session.remainingLikes = 0;
      message = '今日のLikeを使い切りました';
      return;
    }
    message = e.message;
  }

  async function onLike() {
    if (!card || busy || outOfLikes) return;
    const target = card;

    busy = true;
    message = null;
    try {
      const res = await withDayGuard(() => api.post('/api/likes', { persona_id: target.id }));
      session.remainingLikes = res.remaining_likes;
      consumeCurrent();

      if (res.matched) {
        session.matchCount += 1;
        matchedPersona = res.target_persona;
      } else {
        likeFlash = true;
        setTimeout(() => {
          likeFlash = false;
        }, LIKE_FLASH_MS);
      }
      await ensureCards();
    } catch (e) {
      handleActionError(e, target.id);
    } finally {
      busy = false;
    }
  }

  async function onPass() {
    if (!card || busy) return;
    const target = card;

    busy = true;
    message = null;
    try {
      const res = await withDayGuard(() => api.post('/api/passes', { persona_id: target.id }));
      // Once the server excludes the persona for the day there is nothing left
      // for the local cooldown to do.
      consumeCurrent({ cooldownId: res.excluded_for_today ? null : target.id });
      await ensureCards();
    } catch (e) {
      handleActionError(e, target.id);
    } finally {
      busy = false;
    }
  }
</script>

<section class={ui.screen}>
  <header class={ui.header}>
    <button class={ui.back} onclick={goHome}>← ホーム</button>
    <h1 class={ui.title}>探す</h1>
  </header>

  <div class={styles.meta}>
    <span class={styles.likeBudget}>残りLike {session.remainingLikes} / 10</span>
    <span class={styles.receivedBadge}>Likeされた {session.receivedLikeCount}</span>
  </div>

  {#if message}
    <p class={ui.error}>{message}</p>
  {/if}

  {#if card}
    <div class={styles.stage}>
      <PersonaCard persona={card} />
      {#if likeFlash}
        <div class={styles.likeFlash}>LIKE</div>
      {/if}
    </div>

    <div class={ui.actions}>
      <button class={ui.pass} onclick={onPass} disabled={busy}>パス</button>
      <button class={ui.like} onclick={onLike} disabled={busy || outOfLikes}>Like</button>
    </div>
  {:else if discover.loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if discover.error}
    <p class={ui.error}>{discover.error}</p>
  {:else}
    <p class={ui.empty}>いま出会える人がいません。しばらくしてからまた見てみてください。</p>
  {/if}
</section>

{#if matchedPersona}
  <MatchAnimation
    own={session.persona}
    counterpart={matchedPersona}
    onclose={() => (matchedPersona = null)}
  />
{/if}
