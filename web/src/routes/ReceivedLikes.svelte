<script>
  import { onMount } from 'svelte';
  import ui from '../components/ui.module.css';
  import styles from './List.module.css';
  import Icon from '../components/Icon.svelte';
  import PersonaCard from '../components/PersonaCard.svelte';
  import MatchAnimation from '../components/MatchAnimation.svelte';
  import { api, ApiError } from '../lib/api.js';
  import { session, withDayGuard } from '../lib/session.svelte.js';
  import { errorMessage } from '../lib/errors.js';

  let personas = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let message = $state(null);
  let matchedPersona = $state(null);

  // このセッションでの、カードごとの Like お返しの結果。
  let outcome = $state({});

  const outOfLikes = $derived(session.remainingLikes <= 0);

  onMount(async () => {
    try {
      // Likeされた一覧だけでは、その相手にすでに Like を返したか分からない。
      // そのため当日の送信済み Like もあわせて取得する。これをしないと、
      // すでにマッチ済みの相手にも「Likeを返す」ボタンが出てしまう。
      const [received, sent] = await Promise.all([
        api.get('/api/likes/received'),
        api.get('/api/likes/sent'),
      ]);

      personas = received.personas;
      outcome = Object.fromEntries(
        sent.personas.map((p) => [p.id, p.matched ? 'matched' : 'liked']),
      );

      // サーバ側でバッジが消えるのは、この画面を開いたとき。
      session.hasUnseenLikes = false;
      session.receivedLikeCount = personas.length;
    } catch (e) {
      error = errorMessage(e);
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
      }
      message = errorMessage(e);
    }
  }
</script>

<section class={ui.screen}>
  <div class={ui.header}>
    <h1 class={ui.title}>Likeされた</h1>
  </div>

  {#if message}<p class={ui.error}>{message}</p>{/if}

  {#if loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if error}
    <p class={ui.error}>{error}</p>
  {:else if personas.length === 0}
    <p class={ui.empty}>まだLikeされていません。<br />カードを見てもらえるのを待ちましょう。</p>
  {:else}
    <ul class={ui.list}>
      {#each personas as persona (persona.id)}
        <li class={styles.item}>
          <PersonaCard {persona} badge={outcome[persona.id] === 'matched' ? 'MATCH' : null} />
          {#if outcome[persona.id] === 'matched'}
            <p class={styles.done}>マッチしました</p>
          {:else if outcome[persona.id] === 'liked'}
            <p class={styles.done}>Like済み</p>
          {:else}
            <button class={ui.primary} onclick={() => likeBack(persona)} disabled={outOfLikes}>
              <span class={styles.buttonInner}><Icon name="heart" size={16} filled />Likeを返す</span>
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
