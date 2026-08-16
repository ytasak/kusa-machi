<script>
  import { onMount } from 'svelte';
  import styles from './MyPage.module.css';
  import ui from '../components/ui.module.css';
  import Icon from '../components/Icon.svelte';
  import PersonaCard from '../components/PersonaCard.svelte';
  import { session, refreshHome } from '../lib/session.svelte.js';
  import { go, SCREENS } from '../lib/nav.svelte.js';
  import { errorMessage } from '../lib/errors.js';

  let error = $state(null);

  // マイページ is recreated every time the user taps its tab, so this is the
  // "state refreshes on navigation" the spec asks for: counters and badges
  // pick up likes and matches that arrived while another screen was open.
  onMount(async () => {
    try {
      await refreshHome();
    } catch (e) {
      error = errorMessage(e);
    }
  });
</script>

<section class={ui.screen}>
  <div class={ui.header}>
    <h1 class={ui.title}>マイページ</h1>
  </div>

  {#if error}<p class={ui.error}>{error}</p>{/if}

  {#if session.persona}
    <PersonaCard persona={session.persona} variant="hero" badge="あなた" />
  {/if}

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
    <p class={ui.notice}><Icon name="heart" size={15} filled />新しいLikeがあります</p>
  {/if}
  {#if session.hasUnseenMatches}
    <p class={ui.notice}><Icon name="sparkle" size={15} filled />新しいMatchがあります！</p>
  {/if}

  <div class={styles.menu}>
    <button class={styles.menuItem} onclick={() => go(SCREENS.sentLikes)}>
      送信済みLike
      <span class={styles.chevron}><Icon name="chevron" size={18} /></span>
    </button>
    <button class={styles.menuItem} onclick={() => go(SCREENS.profile)}>
      プロフィール編集
      <span class={styles.chevron}><Icon name="chevron" size={18} /></span>
    </button>
  </div>
</section>
