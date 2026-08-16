<script>
  import styles from './StartNewLife.module.css';
  import ui from '../components/ui.module.css';
  import Icon from '../components/Icon.svelte';
  import GeneratingAnimation from '../components/GeneratingAnimation.svelte';
  import { startNewLife } from '../lib/session.svelte.js';
  import { errorMessage } from '../lib/errors.js';
  import { go, SCREENS } from '../lib/nav.svelte.js';
  import { vibrate, HAPTICS } from '../lib/haptics.js';

  // 生成アニメーションを少しのあいだ見せてから、全属性を一度に開示する。
  const REVEAL_DELAY_MS = 1200;

  let generating = $state(false);
  let error = $state(null);

  async function onStart() {
    generating = true;
    error = null;
    vibrate(HAPTICS.start);
    const startedAt = Date.now();
    try {
      await startNewLife();
      const elapsed = Date.now() - startedAt;
      if (elapsed < REVEAL_DELAY_MS) {
        await new Promise((resolve) => setTimeout(resolve, REVEAL_DELAY_MS - elapsed));
      }
      go(SCREENS.mypage);
    } catch (e) {
      error = errorMessage(e);
    } finally {
      generating = false;
    }
  }
</script>

<section class={styles.screen}>
  {#if generating}
    <GeneratingAnimation />
  {:else}
    <div class={styles.mark}>
      <Icon name="sparkle" size={40} filled />
    </div>

    <h1 class={styles.title}>今日の人生</h1>
    <p class={styles.lead}>
      毎日ひとつ、その日だけのあなたが配られます。<br />
      Likeは1日10回。0時にすべてリセットされます。
    </p>

    {#if error}<p class={ui.error}>{error}</p>{/if}

    <button class="{ui.primary} {styles.cta}" onclick={onStart}>新しい人生を始める</button>
  {/if}
</section>
