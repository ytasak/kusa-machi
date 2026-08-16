<script>
  import { onMount } from 'svelte';
  import styles from './App.module.css';
  import ui from './components/ui.module.css';
  import AppHeader from './components/AppHeader.svelte';
  import TabBar from './components/TabBar.svelte';
  import Modal from './components/Modal.svelte';
  import StartNewLife from './routes/StartNewLife.svelte';
  import Discover from './routes/Discover.svelte';
  import ReceivedLikes from './routes/ReceivedLikes.svelte';
  import SentLikes from './routes/SentLikes.svelte';
  import Matches from './routes/Matches.svelte';
  import MyPage from './routes/MyPage.svelte';
  import ProfileEdit from './routes/ProfileEdit.svelte';
  import { session, bootstrap, remainingMsFrom, markDayEnded, startNewLife } from './lib/session.svelte.js';
  import { ticker, startTicker } from './lib/ticker.svelte.js';
  import { nav, SCREENS, go } from './lib/nav.svelte.js';
  import { errorMessage } from './lib/errors.js';

  const FIVE_MINUTES_MS = 5 * 60 * 1000;

  let warningDismissed = $state(false);
  let startingNewLife = $state(false);
  let startError = $state(null);

  onMount(async () => {
    startTicker();
    await bootstrap();
  });

  const remaining = $derived(remainingMsFrom(ticker.now));

  // 時計が JST の 00:00 を過ぎた瞬間にその日は終わる。以降サーバに
  // 何か問い合わせたかどうかは関係ない。
  $effect(() => {
    if (!session.loading && session.gameDate && remaining <= 0) {
      markDayEnded();
    }
  });

  const showFiveMinuteWarning = $derived(
    !session.loading &&
      !session.dayEnded &&
      session.personaGenerated &&
      remaining > 0 &&
      remaining <= FIVE_MINUTES_MS &&
      !warningDismissed,
  );

  async function beginNextLife() {
    startingNewLife = true;
    startError = null;
    try {
      await startNewLife();
      warningDismissed = false;
      go(SCREENS.mypage);
    } catch (e) {
      startError = errorMessage(e);
    } finally {
      startingNewLife = false;
    }
  }
</script>

{#if session.loading}
  <p class={styles.centered}>読み込み中...</p>
{:else if session.error}
  <div class={styles.centered}>
    <p class={ui.error}>{session.error}</p>
    <button class={ui.secondary} onclick={() => bootstrap()}>再読み込み</button>
  </div>
{:else if !session.personaGenerated}
  <!-- 当日の Persona が無ければ市場では何もできないため、開始画面はタブの
       中に収まらず、アプリ全体を占有する。 -->
  <StartNewLife />
{:else}
  <div class={styles.app}>
    <AppHeader />
    <main class={styles.content}>
      {#if nav.screen === SCREENS.receivedLikes}
        <ReceivedLikes />
      {:else if nav.screen === SCREENS.matches}
        <Matches />
      {:else if nav.screen === SCREENS.mypage}
        <MyPage />
      {:else if nav.screen === SCREENS.sentLikes}
        <SentLikes />
      {:else if nav.screen === SCREENS.profile}
        <ProfileEdit />
      {:else}
        <Discover />
      {/if}
    </main>
    <TabBar />
  </div>
{/if}

{#if session.dayEnded}
  <Modal title="今日の人生が終了しました">
    <p class={styles.modalCopy}>新しい人生を始めると、今日のあなたが決まります。</p>
    {#if startError}<p class={ui.error}>{startError}</p>{/if}
    <button class={ui.primary} onclick={beginNextLife} disabled={startingNewLife}>
      新しい人生を始める
    </button>
  </Modal>
{:else if showFiveMinuteWarning}
  <Modal title="今日の人生はあと5分です">
    <p class={styles.modalCopy}>Likeを使い切ってから0時を迎えましょう。</p>
    <button class={ui.secondary} onclick={() => (warningDismissed = true)}>とじる</button>
  </Modal>
{/if}
