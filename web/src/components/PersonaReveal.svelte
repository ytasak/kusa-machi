<script>
  import { onDestroy } from 'svelte';
  import styles from './PersonaReveal.module.css';
  import ui from './ui.module.css';
  import PersonaCard from './PersonaCard.svelte';
  import { revealItems, rankOf, TIER } from '../lib/gacha.js';
  import { vibrate, HAPTICS } from '../lib/haptics.js';

  // 抽選済みの Persona を1属性ずつ開示する全画面演出。
  // persona が null のあいだは抽選中の溜めを見せ、届いた時点で開示が始まる。
  let { persona = null, error = null, onretry, onclose } = $props();

  /** 回転中に表示を差し替える間隔。速すぎると読めず、遅いと止まって見える。 */
  const SPIN_TICK_MS = 55;
  /** 項目ごとの回転時間。後半ほど長く溜める。REVEAL_STEPS と同じ並び。 */
  const SPIN_MS = [420, 320, 500, 640, 780, 1000];
  /** ガチャの慣習どおり、当たりのときだけ演出を引き延ばす。 */
  const RARE_EXTRA_MS = 650;
  /** 確定を見せてから次へ進むまでの間。 */
  const HOLD_MS = 800;

  const reduceMotion =
    typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

  const SHAPES = ['✦', '★', '♥', '✧'];
  const COLORS = ['#ffd166', '#ff8fae', '#66e0ff', '#b98cff', '#fff'];

  // 抽選結果が届いた時点で確定する。開示が始まる前の初回描画でも
  // 1件目を参照できるよう、$effect ではなくここで導出しておく。
  const items = $derived(persona ? revealItems(persona) : []);

  let index = $state(0);
  /** 'spin' 回転中 / 'lock' 確定を見せている / 'done' 全部出し終えた */
  let phase = $state('spin');
  let spinText = $state('');
  /** 弾ける粒。撒くたびに作り直す。 */
  let burst = $state(null);

  let tapEl = $state(null);
  let ctaEl = $state(null);

  let tickTimer = null;
  let stepTimer = null;
  let started = false;

  const current = $derived(items[index] ?? null);
  const rank = $derived(phase === 'done' && items.length > 0 ? rankOf(items) : null);
  /** ステップ表示の進み具合。確定した瞬間に1つ増える。 */
  const doneCount = $derived(phase === 'spin' ? index : index + 1);
  /** 画面全体の光り方。確定表示のあいだだけレア度を反映する。 */
  const glow = $derived(rank?.tier ?? (phase === 'lock' ? (current?.tier ?? 0) : 0));

  const shown = $derived(phase === 'spin' ? spinText : (current?.value ?? ''));
  const valueSize = $derived(sizeFor(current?.chars ?? 4));

  // 抽選結果が届いたら開示を始める。人生は1日1回なので二度は走らない。
  $effect(() => {
    if (items.length > 0 && !started) {
      started = true;
      beginStep(0);
    }
  });

  // オーバーレイが開いているあいだ、キーボード操作の起点を演出の中に置く。
  $effect(() => {
    (ctaEl ?? tapEl)?.focus();
  });

  onDestroy(clearTimers);

  function clearTimers() {
    clearInterval(tickTimer);
    clearTimeout(stepTimer);
    tickTimer = null;
    stepTimer = null;
  }

  function sizeFor(chars) {
    if (chars <= 3) return 48;
    if (chars <= 5) return 40;
    return 30;
  }

  function beginStep(i) {
    clearTimers();
    index = i;
    phase = 'spin';
    burst = null;

    const item = items[i];
    spinText = item.spin();

    // 動きを減らす設定では回転そのものを省き、順に出すだけにする。
    if (reduceMotion) {
      lockStep();
      return;
    }
    tickTimer = setInterval(() => (spinText = item.spin()), SPIN_TICK_MS);
    stepTimer = setTimeout(lockStep, SPIN_MS[i] + (item.tier > TIER.normal ? RARE_EXTRA_MS : 0));
  }

  function lockStep() {
    if (phase !== 'spin') return;
    clearTimers();
    phase = 'lock';

    const item = items[index];
    vibrate(item.tier > TIER.normal ? HAPTICS.revealRare : HAPTICS.reveal);
    if (item.tier === TIER.legend) spawnBurst(24);

    // 動きを減らす設定でも1項目ずつ読める間は残す。急がせるのが目的ではない。
    stepTimer = setTimeout(advance, reduceMotion ? 550 : HOLD_MS);
  }

  function advance() {
    if (phase !== 'lock') return;
    clearTimers();
    if (index + 1 < items.length) {
      beginStep(index + 1);
    } else {
      finish();
    }
  }

  function finish() {
    clearTimers();
    index = items.length - 1;
    phase = 'done';

    const result = rankOf(items);
    vibrate(HAPTICS.rank);
    if (result.tier > TIER.normal) spawnBurst(20 + result.score * 6);
  }

  /** 画面のタップ。回転中なら即停止、確定表示中なら次へ。連打で飛ばせる。 */
  function onTap() {
    if (phase === 'spin') lockStep();
    else if (phase === 'lock') advance();
  }

  function skipAll() {
    finish();
  }

  function spawnBurst(count) {
    if (reduceMotion) return;
    burst = { id: `${index}-${phase}`, particles: makeParticles(count) };
  }

  /** 中心から弾ける粒。撒くたびに違う散り方になるよう毎回引き直す。 */
  function makeParticles(count) {
    return Array.from({ length: count }, () => {
      const angle = Math.random() * Math.PI * 2;
      const distance = 90 + Math.random() * 200;
      return {
        shape: SHAPES[Math.floor(Math.random() * SHAPES.length)],
        color: COLORS[Math.floor(Math.random() * COLORS.length)],
        tx: Math.cos(angle) * distance,
        ty: Math.sin(angle) * distance,
        scale: 0.6 + Math.random() * 1.2,
        rotate: -180 + Math.random() * 360,
        delay: Math.random() * 140,
        duration: 800 + Math.random() * 520,
      };
    });
  }
</script>

<div
  class={styles.backdrop}
  data-tier={glow}
  data-stage={persona && !error ? 'reveal' : 'charge'}
  role="dialog"
  aria-modal="true"
  aria-label="新しい人生の抽選"
>
  <div class={styles.rays} aria-hidden="true"></div>

  {#if burst}
    {#key burst.id}
      <div class={styles.burst} aria-hidden="true">
        {#each burst.particles as p, i (i)}
          <span
            class={styles.particle}
            style="--tx:{p.tx}px; --ty:{p.ty}px; --sc:{p.scale}; --rot:{p.rotate}deg;
                   --delay:{p.delay}ms; --dur:{p.duration}ms; color:{p.color}"
          >
            {p.shape}
          </span>
        {/each}
      </div>
    {/key}
  {/if}

  {#if !error && persona && phase !== 'done'}
    <!-- 画面のどこを触っても進む。ガチャは連打できてこそ気持ちがいい。 -->
    <button
      bind:this={tapEl}
      class={styles.tap}
      onclick={onTap}
      aria-label={phase === 'spin' ? 'いますぐ止める' : '次へ'}
    ></button>
  {/if}

  <div class={styles.content}>
    {#if error}
      <p class={ui.error}>{error}</p>
      <button bind:this={ctaEl} class="{ui.primary} {styles.cta}" onclick={onretry}>
        もう一度引く
      </button>
      <button class={styles.skip} onclick={onclose}>とじる</button>
    {:else if !persona}
      <div class={styles.orbWrap} aria-hidden="true">
        <span class={styles.orbRing}></span>
        <span class={styles.orb}></span>
      </div>
      <p class={styles.charging}>人生を抽選しています</p>
    {:else if phase === 'done'}
      <p class={styles.rankLabel}>総合レア度</p>
      <p class={styles.rank}>{rank.label}</p>
      <p class={styles.rankCopy}>{rank.copy}</p>

      <div class={styles.cardSlot}>
        <PersonaCard {persona} variant="hero" badge="あなた" />
      </div>

      <button bind:this={ctaEl} class="{ui.primary} {styles.cta}" onclick={onclose}>
        この人生を始める
      </button>
    {:else}
      <ol class={styles.pips} aria-hidden="true">
        {#each items as item, i (item.key)}
          <li class="{styles.pip} {i < doneCount ? styles.pipOn : ''}"></li>
        {/each}
      </ol>

      <div class={styles.stage}>
        <p class={styles.stepLabel}>{current.label}</p>

        <div class="{styles.frame} {phase === 'lock' ? styles.frameLocked : ''}">
          <span
            class="{styles.value} {phase === 'spin' ? styles.spinning : ''}"
            style="font-size:{valueSize}px"
            aria-live="polite"
          >
            {shown}
          </span>
        </div>

        <div class={styles.tierSlot}>
          {#if phase === 'lock' && current.tier > TIER.normal}
            <span
              class="{styles.tierBadge} {current.tier === TIER.legend ? styles.legendBadge : ''}"
            >
              ✦ {current.tier === TIER.legend ? '激レア' : 'レア'} ✦
            </span>
            {#if current.note}<span class={styles.note}>{current.note}</span>{/if}
          {/if}
        </div>
      </div>

      <!-- 未開示のぶんも枠だけ先に置く。残りが何個あるかが見え、
           確定するたびにレイアウトが上下に跳ねるのも防げる。 -->
      <ul class={styles.ledger}>
        {#each items as item, i (item.key)}
          <li class={styles.ledgerRow} data-tier={i < doneCount ? item.tier : 0}>
            <span class={styles.ledgerLabel}>{item.label}</span>
            {#if i < doneCount}
              <span class={styles.ledgerValue}>{item.value}</span>
            {:else}
              <span class={styles.ledgerPending}>?</span>
            {/if}
          </li>
        {/each}
      </ul>

      <div class={styles.footer}>
        <p class={styles.hint}>{phase === 'spin' ? 'タップで止める' : 'タップで次へ'}</p>
        <button class={styles.skip} onclick={skipAll}>すべて表示</button>
      </div>
    {/if}
  </div>
</div>
