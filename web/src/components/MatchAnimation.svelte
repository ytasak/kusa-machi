<script>
  import styles from './MatchAnimation.module.css';
  import PersonaCard from './PersonaCard.svelte';

  // Shows both personas together, as the spec's match animation requires.
  let { own, counterpart, onclose } = $props();
</script>

<div class={styles.backdrop} role="dialog" aria-modal="true" aria-label="マッチしました">
  <div class={styles.panel}>
    <p class={styles.title}>MATCH!</p>
    <p class={styles.copy}>今日の人生でマッチしました</p>

    <div class={styles.pair}>
      {#if own}
        <PersonaCard persona={own} badge="あなた" />
      {/if}
      <PersonaCard persona={counterpart} />
    </div>

    <button class={styles.close} onclick={onclose}>とじる</button>
  </div>
</div>
