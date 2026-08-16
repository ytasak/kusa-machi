<script>
  import styles from './PersonaCard.module.css';
  import { ageAndGender, height, income } from '../lib/format.js';

  // One shared card for Discover, Received Likes, Sent Likes and Matches.
  // Only the badge differs between screens.
  let { persona, badge = null } = $props();
</script>

<article class={styles.card}>
  {#if badge}
    <span class={styles.badge}>{badge}</span>
  {/if}

  {#if persona.name}
    <h2 class={styles.name}>{persona.name}</h2>
  {/if}

  <dl class={styles.rows}>
    <dt class={styles.label}>年齢・性別</dt>
    <dd class={styles.value}>{ageAndGender(persona)}</dd>

    <dt class={styles.label}>身長</dt>
    <dd class={styles.value}>{height(persona)}</dd>

    <dt class={styles.label}>職業</dt>
    <dd class={styles.value}>{persona.occupation}</dd>

    <dt class={styles.label}>年収</dt>
    <dd class={styles.value}>{income(persona)}</dd>

    <dt class={styles.label}>学歴</dt>
    <dd class={styles.value}>{persona.education}</dd>

    {#if persona.hobby}
      <dt class={styles.label}>趣味</dt>
      <dd class={styles.value}>{persona.hobby}</dd>
    {/if}

    {#if persona.bio}
      <dt class={styles.label}>ひとこと</dt>
      <dd class={styles.value}>{persona.bio}</dd>
    {/if}
  </dl>
</article>
