<script>
  import styles from './PersonaCard.module.css';
  import Avatar from './Avatar.svelte';
  import { ageAndGender, height, income } from '../lib/format.js';

  // One shared card for Discover, Received Likes, Sent Likes and Matches.
  // `variant` only changes the density; the attribute order is always the one
  // the spec fixes: name, age+gender, height, occupation, income, education,
  // hobby, bio. Unset fields are omitted entirely.
  let { persona, badge = null, variant = 'row' } = $props();

  const isHero = $derived(variant === 'hero');
</script>

<article class="{styles.card} {isHero ? styles.hero : styles.row}">
  {#if badge}
    <span class={styles.badge}>{badge}</span>
  {/if}

  <div class={styles.head}>
    <Avatar {persona} size={isHero ? 96 : 52} />
    <div class={styles.identity}>
      {#if persona.name}
        <h2 class={styles.name}>{persona.name}</h2>
      {/if}
      <p class={styles.headline}>{ageAndGender(persona)}</p>
    </div>
  </div>

  <div class={styles.tags}>
    <span class={styles.tag}><span class={styles.tagLabel}>身長</span>{height(persona)}</span>
    <span class={styles.tag}><span class={styles.tagLabel}>職業</span>{persona.occupation}</span>
    <span class={styles.tag}><span class={styles.tagLabel}>年収</span>{income(persona)}</span>
    <span class={styles.tag}><span class={styles.tagLabel}>学歴</span>{persona.education}</span>
  </div>

  {#if persona.hobby || persona.bio}
    <div class={styles.notes}>
      {#if persona.hobby}
        <div class={styles.note}>
          <span class={styles.noteLabel}>趣味</span>
          <span class={styles.noteValue}>{persona.hobby}</span>
        </div>
      {/if}
      {#if persona.bio}
        <div class={styles.note}>
          <span class={styles.noteLabel}>ひとこと</span>
          <span class={styles.noteValue}>{persona.bio}</span>
        </div>
      {/if}
    </div>
  {/if}
</article>
