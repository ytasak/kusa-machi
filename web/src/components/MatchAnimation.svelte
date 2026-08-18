<script>
  import styles from './MatchAnimation.module.css';
  import PersonaCard from './PersonaCard.svelte';

  // 仕様の Match アニメーションが求めるとおり、両者の Persona を並べて見せる。
  // likesGained は Match 報酬で実際に増えた Like の数。所持上限で何も増えな
  // かったときは 0 で来るので、その場合は回復の行を出さない。
  let { own, counterpart, likesGained = 0, onclose } = $props();

  const SHAPES = ['♥', '♥', '♥', '✦', '★'];
  const COLORS = ['#ff5470', '#ffd166', '#ff8fae', '#66e0ff', '#fff', '#ffb03a'];

  /**
   * 中心から弾ける粒。開くたびに違う散り方になるよう、マウント時に一度だけ
   * 乱数で決める。
   */
  const particles = Array.from({ length: 34 }, () => {
    const angle = Math.random() * Math.PI * 2;
    const distance = 120 + Math.random() * 220;
    return {
      shape: SHAPES[Math.floor(Math.random() * SHAPES.length)],
      color: COLORS[Math.floor(Math.random() * COLORS.length)],
      tx: Math.cos(angle) * distance,
      ty: Math.sin(angle) * distance - 60, // わずかに上へ散らす
      scale: 0.7 + Math.random() * 1.3,
      rotate: -180 + Math.random() * 360,
      delay: Math.random() * 180,
      duration: 900 + Math.random() * 600,
    };
  });
</script>

<div class={styles.backdrop} role="dialog" aria-modal="true" aria-label="マッチしました">
  <div class={styles.burst} aria-hidden="true">
    {#each particles as p, i (i)}
      <span
        class={styles.particle}
        style="--tx:{p.tx}px; --ty:{p.ty}px; --sc:{p.scale}; --rot:{p.rotate}deg;
               --delay:{p.delay}ms; --dur:{p.duration}ms; color:{p.color}"
      >
        {p.shape}
      </span>
    {/each}
  </div>

  <div class={styles.panel}>
    <p class={styles.title}>MATCH!</p>
    <p class={styles.copy}>今日の人生でマッチしました</p>

    {#if likesGained > 0}
      <p class={styles.reward}>Match成立！ Like +{likesGained}</p>
    {/if}

    <div class={styles.pair}>
      {#if own}
        <PersonaCard persona={own} badge="あなた" />
      {/if}
      <PersonaCard persona={counterpart} />
    </div>

    <button class={styles.close} onclick={onclose}>とじる</button>
  </div>
</div>
