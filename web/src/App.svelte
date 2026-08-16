<script>
  import { onMount } from 'svelte';
  import styles from './App.module.css';
  import ui from './components/ui.module.css';
  import Modal from './components/Modal.svelte';
  import Home from './routes/Home.svelte';
  import Discover from './routes/Discover.svelte';
  import ReceivedLikes from './routes/ReceivedLikes.svelte';
  import SentLikes from './routes/SentLikes.svelte';
  import Matches from './routes/Matches.svelte';
  import ProfileEdit from './routes/ProfileEdit.svelte';
  import { session, bootstrap, remainingMsFrom, markDayEnded, startNewLife } from './lib/session.svelte.js';
  import { ticker, startTicker } from './lib/ticker.svelte.js';
  import { nav, SCREENS, goHome } from './lib/nav.svelte.js';

  const FIVE_MINUTES_MS = 5 * 60 * 1000;

  let warningDismissed = $state(false);
  let startingNewLife = $state(false);
  let startError = $state(null);

  onMount(async () => {
    startTicker();
    await bootstrap();
  });

  const remaining = $derived(remainingMsFrom(ticker.now));

  // The day is over the moment the clock passes 00:00 JST, whether or not the
  // server has been asked anything since.
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
      goHome();
    } catch (e) {
      startError = e.message;
    } finally {
      startingNewLife = false;
    }
  }
</script>

<main class={styles.shell}>
  {#if session.loading}
    <p class={styles.centered}>読み込み中...</p>
  {:else if session.error}
    <div class={styles.centered}>
      <p class={ui.error}>{session.error}</p>
      <button class={styles.modalButton} onclick={() => bootstrap()}>再読み込み</button>
    </div>
  {:else if nav.screen === SCREENS.discover}
    <Discover />
  {:else if nav.screen === SCREENS.receivedLikes}
    <ReceivedLikes />
  {:else if nav.screen === SCREENS.sentLikes}
    <SentLikes />
  {:else if nav.screen === SCREENS.matches}
    <Matches />
  {:else if nav.screen === SCREENS.profile}
    <ProfileEdit />
  {:else}
    <Home />
  {/if}
</main>

{#if session.dayEnded}
  <Modal title="今日の人生が終了しました">
    <p class={styles.modalCopy}>新しい人生を始めると、今日のあなたが決まります。</p>
    {#if startError}<p class={ui.error}>{startError}</p>{/if}
    <button class={styles.modalButton} onclick={beginNextLife} disabled={startingNewLife}>
      新しい人生を始める
    </button>
  </Modal>
{:else if showFiveMinuteWarning}
  <Modal title="今日の人生はあと5分です">
    <button class={styles.modalSecondary} onclick={() => (warningDismissed = true)}>とじる</button>
  </Modal>
{/if}
