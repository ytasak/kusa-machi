<script>
  import { onMount } from 'svelte';
  import styles from './App.module.css';
  import ui from './components/ui.module.css';
  import AppHeader from './components/AppHeader.svelte';
  import TabBar from './components/TabBar.svelte';
  import Modal from './components/Modal.svelte';
  import StartNewLife from './routes/StartNewLife.svelte';
  import CookieBlocked from './routes/CookieBlocked.svelte';
  import Discover from './routes/Discover.svelte';
  import ReceivedLikes from './routes/ReceivedLikes.svelte';
  import SentLikes from './routes/SentLikes.svelte';
  import Matches from './routes/Matches.svelte';
  import MyPage from './routes/MyPage.svelte';
  import ProfileEdit from './routes/ProfileEdit.svelte';
  import PersonaReveal from './components/PersonaReveal.svelte';
  import {
    session,
    bootstrap,
    remainingMsFrom,
    nextRecoveryMsFrom,
    refreshHome,
    markDayEnded,
  } from './lib/session.svelte.js';
  import { ticker, startTicker } from './lib/ticker.svelte.js';
  import { nav, SCREENS } from './lib/nav.svelte.js';
  import { reveal, rollNewLife, closeReveal } from './lib/reveal.svelte.js';

  const FIVE_MINUTES_MS = 5 * 60 * 1000;

  let warningDismissed = $state(false);

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

  // 時間回復のタイマーが満了したら、一度だけホームを読み直して残数に反映する。
  // 起きるのは3時間に1回なのでポーリングにはならず、サーバ側にも常駐の
  // タイマーを置かずに済む。同じ満了で何度も投げないよう、投げた時刻を覚える。
  let reloadedFor = null;

  $effect(() => {
    if (session.loading || session.dayEnded) return;

    const untilNext = nextRecoveryMsFrom(ticker.now);
    if (untilNext === null || untilNext > 0) return;
    if (reloadedFor === session.nextRecoveryAt) return;

    reloadedFor = session.nextRecoveryAt;
    // 失敗しても次のティックで投げ直せるほどのものではないので、黙って諦める。
    // 画面遷移のたびにホームは読み直されるため、遅くともそこで追いつく。
    refreshHome().catch(() => {});
  });

  const showFiveMinuteWarning = $derived(
    !session.loading &&
      !session.dayEnded &&
      session.personaGenerated &&
      remaining > 0 &&
      remaining <= FIVE_MINUTES_MS &&
      // 開示演出の最中に日付が変わっても、演出の上に警告を重ねない。
      !reveal.active &&
      !warningDismissed,
  );

  // 日をまたいで新しい人生に入るので、その日の5分前警告を出し直せるようにする。
  function beginNextLife() {
    warningDismissed = false;
    rollNewLife();
  }
</script>

{#if session.loading}
  <p class={styles.centered}>読み込み中...</p>
{:else if session.error}
  <div class={styles.centered}>
    <p class={ui.error}>{session.error}</p>
    <button class={ui.secondary} onclick={() => bootstrap()}>再読み込み</button>
  </div>
{:else if session.cookiesBlocked}
  <!-- Cookie が保存されない状態では何をしても CSRF で弾かれる。先に案内する。 -->
  <CookieBlocked />
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
    <button class={ui.primary} onclick={beginNextLife}>新しい人生を始める</button>
  </Modal>
{:else if showFiveMinuteWarning}
  <Modal title="今日の人生はあと5分です">
    <p class={styles.modalCopy}>Likeを使い切ってから0時を迎えましょう。</p>
    <button class={ui.secondary} onclick={() => (warningDismissed = true)}>とじる</button>
  </Modal>
{/if}

<!-- 抽選の溜めから開示までを引き受けるオーバーレイ。開始画面からでも
     「今日の人生が終了しました」からでも、ここに着地する。 -->
{#if reveal.active}
  <PersonaReveal
    persona={reveal.persona}
    error={reveal.error}
    onretry={rollNewLife}
    onclose={closeReveal}
  />
{/if}
