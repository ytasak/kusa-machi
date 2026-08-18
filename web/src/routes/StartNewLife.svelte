<script>
  import styles from './StartNewLife.module.css';
  import ui from '../components/ui.module.css';
  import Icon from '../components/Icon.svelte';
  import { rollNewLife } from '../lib/reveal.svelte.js';
  import { vibrate, HAPTICS } from '../lib/haptics.js';

  // 抽選中の表示も失敗時の案内も PersonaReveal のオーバーレイが引き受けるので、
  // この画面はボタンを出すだけでよい。
  function onStart() {
    vibrate(HAPTICS.start);
    rollNewLife();
  }
</script>

<section class={styles.screen}>
  <div class={styles.mark}>
    <Icon name="sparkle" size={40} filled />
  </div>

  <h1 class={styles.title}>クサマチ</h1>
  <p class={styles.tagline}>今日の人生を、ひとつ。</p>
  <div class={styles.lead}>
    <p>毎日ひとつ、その日だけのあなたが配られます。</p>
    <p>Likeは1日10回。0時にすべて消えます。</p>
    <p>プロフィール完成で+1、Match成立で+2だけ戻ります。</p>
  </div>

  <button class="{ui.primary} {styles.cta}" onclick={onStart}>新しい人生を始める</button>
</section>
