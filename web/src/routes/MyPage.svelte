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

  // マイページはタブを押すたびに作り直されるため、これが仕様の言う
  // 「画面遷移で状態を再取得する」にあたる。別の画面を開いているあいだに
  // 届いた Like や Match がカウンタとバッジに反映される。
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
      <span class={styles.statValue}>{session.remainingLikes} / {session.likeCapacity}</span>
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

  <!-- 報酬は完成させた後にしか見えないと気づけない。残りLike のすぐ下に置いて、
       「何をすれば増えるのか」を数字と並べて示す。押すとそのまま編集画面へ。 -->
  {#if session.profileRewardAvailable}
    <button class={styles.reward} onclick={() => go(SCREENS.profile)}>
      <Icon name="heart" size={15} filled />
      <span class={styles.rewardText}>プロフィールを完成させると <strong>Like +1</strong></span>
      <span class={styles.chevron}><Icon name="chevron" size={18} /></span>
    </button>
  {/if}

  {#if session.hasUnseenLikes}
    <p class={ui.notice}><Icon name="heart" size={15} filled />新しいLikeがあります</p>
  {/if}
  {#if session.hasUnseenMatches}
    <p class={ui.notice}><Icon name="sparkle" size={15} filled />新しいMatchがあります！</p>
  {/if}

  <!-- 送信済みLike は下部ナビのタブに昇格したので、ここには置かない。 -->
  <div class={styles.menu}>
    <button class={styles.menuItem} onclick={() => go(SCREENS.profile)}>
      プロフィール編集
      <span class={styles.chevron}><Icon name="chevron" size={18} /></span>
    </button>
  </div>
</section>
