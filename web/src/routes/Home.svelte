<script>
  import styles from './Home.module.css';
  import ui from '../components/ui.module.css';
  import PersonaCard from '../components/PersonaCard.svelte';
  import GeneratingAnimation from '../components/GeneratingAnimation.svelte';
  import { session, startNewLife, remainingMsFrom } from '../lib/session.svelte.js';
  import { ticker } from '../lib/ticker.svelte.js';
  import { countdown } from '../lib/format.js';
  import { go, SCREENS } from '../lib/nav.svelte.js';

  // The generation animation runs for a moment, then every attribute is
  // revealed at once.
  const REVEAL_DELAY_MS = 1200;

  let generating = $state(false);
  let error = $state(null);

  const remaining = $derived(countdown(remainingMsFrom(ticker.now)));

  async function onStartNewLife() {
    generating = true;
    error = null;
    const startedAt = Date.now();
    try {
      await startNewLife();
      const elapsed = Date.now() - startedAt;
      if (elapsed < REVEAL_DELAY_MS) {
        await new Promise((resolve) => setTimeout(resolve, REVEAL_DELAY_MS - elapsed));
      }
    } catch (e) {
      error = e.message;
    } finally {
      generating = false;
    }
  }
</script>

<section class={ui.screen}>
  <div class={styles.hero}>
    <h1 class={ui.title}>今日の人生</h1>
    <p class={styles.countdown}>今日の人生 残り {remaining}</p>
  </div>

  {#if generating}
    <GeneratingAnimation />
  {:else if !session.personaGenerated}
    <div class={styles.startWrap}>
      <p class={styles.lead}>今日のあなたはまだ決まっていません。</p>
      <button class={ui.primary} onclick={onStartNewLife}>新しい人生を始める</button>
      {#if error}<p class={ui.error}>{error}</p>{/if}
    </div>
  {:else}
    <PersonaCard persona={session.persona} badge="あなた" />

    <div class={styles.stats}>
      <div class={styles.stat}>
        <span class={styles.statLabel}>残りLike</span>
        <span class={styles.statValue}>{session.remainingLikes} / 10</span>
      </div>
      <div class={styles.stat}>
        <span class={styles.statLabel}>Likeされた</span>
        <span class={styles.statValue}>{session.receivedLikeCount}</span>
      </div>
      <div class={styles.stat}>
        <span class={styles.statLabel}>Match</span>
        <span class={styles.statValue}>{session.matchCount}</span>
      </div>
    </div>

    {#if session.hasUnseenLikes}
      <p class={ui.notice}>新しいLikeがあります</p>
    {/if}
    {#if session.hasUnseenMatches}
      <p class={ui.notice}>新しいMatchがあります！</p>
    {/if}

    <nav class={styles.nav}>
      <button class={styles.navItem} onclick={() => go(SCREENS.discover)}>探す</button>
      <button class={styles.navItem} onclick={() => go(SCREENS.receivedLikes)}>
        Likeされた
        {#if session.hasUnseenLikes}<span class={styles.dot}></span>{/if}
      </button>
      <button class={styles.navItem} onclick={() => go(SCREENS.matches)}>
        Match
        {#if session.hasUnseenMatches}<span class={styles.dot}></span>{/if}
      </button>
      <button class={styles.navItem} onclick={() => go(SCREENS.sentLikes)}>送信済みLike</button>
      <button class={styles.navItem} onclick={() => go(SCREENS.profile)}>プロフィール編集</button>
    </nav>
  {/if}
</section>
