<script>
  import { onMount } from 'svelte';
  import ui from '../components/ui.module.css';
  import styles from './MatchDetail.module.css';
  import Icon from '../components/Icon.svelte';
  import PersonaCard from '../components/PersonaCard.svelte';
  import ChildCard from '../components/ChildCard.svelte';
  import GachaReveal from '../components/GachaReveal.svelte';
  import { api } from '../lib/api.js';
  import { withDayGuard } from '../lib/session.svelte.js';
  import { nav, goMatches } from '../lib/nav.svelte.js';
  import { errorMessage } from '../lib/errors.js';
  import { childRevealItems, CHILD_REVEAL_SPIN_MS } from '../lib/gacha.js';
  import { vibrate, HAPTICS } from '../lib/haptics.js';

  // Match 1件の詳細。Match 成立演出で「あとで見る」を選んでも、一覧から
  // ここへ来れば当日中はいつでも子ガチャを回収できる。
  //
  // 子は1 Match につき1人だけで、引き直しはできない。だから「引き直す」に
  // 当たるボタンはこの画面のどこにも置かない。

  /** 抽選を待たせる最短時間。即答されても溜めが無いとガチャにならない。 */
  const MIN_ROLL_MS = 700;

  const matchID = nav.matchID;

  let detail = $state(null);
  let loading = $state(true);
  let error = $state(null);

  /** 子ガチャの演出を出しているあいだ true。 */
  let revealing = $state(false);
  /** 演出の中で開示する子。届くまでは null で、そのあいだは溜めを見せる。 */
  let revealChild = $state(null);
  let revealError = $state(null);
  /** 送信中フラグ。押しっぱなしでも2回投げない。 */
  let drawing = false;

  const items = $derived(revealChild ? childRevealItems(revealChild) : []);

  onMount(async () => {
    // 「この2人の子を引く」から入ってきた場合は、読み込みに続けてそのまま引く。
    // 一度読んだら下ろす。戻ってきたときに勝手に始まらないようにするため。
    const autoDraw = nav.matchAutoDraw;
    nav.matchAutoDraw = false;

    try {
      detail = await api.get(`/api/matches/${matchID}`);
    } catch (e) {
      error = errorMessage(e);
      return;
    } finally {
      loading = false;
    }

    if (autoDraw && !detail.child_generated) drawChild();
  });

  /**
   * 子ガチャを引く。
   *
   * サーバ側は冪等なので、多重に届いても引き直しにはならず同じ子が返る。
   * ここで送信中を見ているのは、演出を二重に開かないためだけ。
   */
  async function drawChild() {
    if (drawing || detail?.child_generated) return;
    drawing = true;

    revealing = true;
    revealChild = null;
    revealError = null;
    vibrate(HAPTICS.reveal);

    const startedAt = Date.now();
    try {
      const child = await withDayGuard(() => api.post(`/api/matches/${matchID}/child`));

      const elapsed = Date.now() - startedAt;
      if (elapsed < MIN_ROLL_MS) {
        await new Promise((resolve) => setTimeout(resolve, MIN_ROLL_MS - elapsed));
      }

      // 画面の側も生成済みにする。これで CTA が消え、演出を閉じたあとは
      // 静的な子カードだけが残る。
      detail = { ...detail, child_generated: true, child };
      revealChild = child;
    } catch (e) {
      revealError = errorMessage(e);
    } finally {
      drawing = false;
    }
  }

  function closeReveal() {
    revealing = false;
    revealChild = null;
    revealError = null;
  }
</script>

<section class={ui.screen}>
  <div class={ui.header}>
    <button class={ui.back} onclick={goMatches} aria-label="Match一覧に戻る">
      <Icon name="back" size={18} />
    </button>
    <h1 class={ui.title}>Match</h1>
  </div>

  {#if loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if error}
    <p class={ui.error}>{error}</p>
  {:else}
    <!-- Match 演出の短縮版。成立の瞬間ほど派手にはせず、2人が並んでいる
         ことだけを静かに見せる。 -->
    <p class={styles.matched}>MATCH!</p>

    <div class={styles.pair}>
      <PersonaCard persona={detail.own_persona} badge="あなた" />
      <PersonaCard persona={detail.target_persona} />
    </div>

    <div class={styles.child}>
      <h2 class={styles.childTitle}>二人の子</h2>

      {#if detail.child_generated}
        <ChildCard child={detail.child} />
        <p class={styles.note}>この子は今日だけの結果です。0時に消えます。</p>
      {:else}
        <p class={styles.lead}>2人の属性を少し引き継いだ人生を1回だけ引けます。</p>
        <button class={ui.primary} onclick={drawChild}>この2人の子を引く</button>
        <p class={styles.note}>引けるのは1回だけ。引き直しはできません。</p>
      {/if}
    </div>
  {/if}
</section>

{#if revealing}
  <GachaReveal
    {items}
    error={revealError}
    label="二人の子の抽選"
    chargingLabel="二人の子を抽選しています"
    ctaLabel="とじる"
    spinMs={CHILD_REVEAL_SPIN_MS}
    holdMs={600}
    onclose={closeReveal}
  >
    {#snippet result()}
      <ChildCard child={revealChild} />
    {/snippet}
  </GachaReveal>
{/if}
