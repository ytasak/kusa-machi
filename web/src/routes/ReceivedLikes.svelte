<script>
  import { onMount } from 'svelte';
  import ui from '../components/ui.module.css';
  import styles from './List.module.css';
  import PersonaCard from '../components/PersonaCard.svelte';
  import MatchAnimation from '../components/MatchAnimation.svelte';
  import { api, ApiError } from '../lib/api.js';
  import { session, withDayGuard } from '../lib/session.svelte.js';
  import { goHome } from '../lib/nav.svelte.js';

  let personas = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let message = $state(null);
  let matchedPersona = $state(null);

  // Per-card outcome of a like-back in this session.
  let outcome = $state({});

  const outOfLikes = $derived(session.remainingLikes <= 0);

  onMount(async () => {
    try {
      const res = await api.get('/api/likes/received');
      personas = res.personas;
      // Opening the screen is what clears the badge server-side.
      session.hasUnseenLikes = false;
      session.receivedLikeCount = personas.length;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  async function likeBack(persona) {
    if (outcome[persona.id] || outOfLikes) return;
    message = null;
    try {
      const res = await withDayGuard(() => api.post('/api/likes', { persona_id: persona.id }));
      session.remainingLikes = res.remaining_likes;
      if (res.matched) {
        session.matchCount += 1;
        outcome = { ...outcome, [persona.id]: 'matched' };
        matchedPersona = res.target_persona;
      } else {
        outcome = { ...outcome, [persona.id]: 'liked' };
      }
    } catch (e) {
      if (e instanceof ApiError && e.code === 'AlreadyLiked') {
        outcome = { ...outcome, [persona.id]: 'liked' };
        return;
      }
      if (e instanceof ApiError && e.code === 'LikeLimitExceeded') {
        session.remainingLikes = 0;
        message = '今日のLikeを使い切りました';
        return;
      }
      message = e.message;
    }
  }

  function badgeFor(persona) {
    return outcome[persona.id] === 'matched' ? 'MATCH' : null;
  }
</script>

<section class={ui.screen}>
  <header class={ui.header}>
    <button class={ui.back} onclick={goHome}>← ホーム</button>
    <h1 class={ui.title}>Likeされた</h1>
  </header>

  {#if message}<p class={ui.error}>{message}</p>{/if}

  {#if loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if error}
    <p class={ui.error}>{error}</p>
  {:else if personas.length === 0}
    <p class={ui.empty}>まだLikeされていません。</p>
  {:else}
    <ul class={ui.list}>
      {#each personas as persona (persona.id)}
        <li class={styles.item}>
          <PersonaCard {persona} badge={badgeFor(persona)} />
          {#if outcome[persona.id] === 'matched'}
            <p class={styles.done}>マッチしました</p>
          {:else if outcome[persona.id] === 'liked'}
            <p class={styles.done}>Like済み</p>
          {:else}
            <button class={ui.like} onclick={() => likeBack(persona)} disabled={outOfLikes}>
              Likeを返す
            </button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>

{#if matchedPersona}
  <MatchAnimation
    own={session.persona}
    counterpart={matchedPersona}
    onclose={() => (matchedPersona = null)}
  />
{/if}
