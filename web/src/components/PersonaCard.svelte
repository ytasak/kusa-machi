<script>
  import styles from './PersonaCard.module.css';
  import Avatar from './Avatar.svelte';
  import { ageAndGender, height, income } from '../lib/format.js';

  // 探す / Likeされた / 送信済みLike / Match で共通のカード。
  // variant が変えるのは情報密度だけで、属性の並びは常に仕様どおり:
  // 名前、年齢+性別、身長、職業、年収、学歴、趣味、ひとこと。
  // 未設定の項目は丸ごと省く。
  let { persona, badge = null, variant = 'row' } = $props();

  const isHero = $derived(variant === 'hero');
</script>

<article class="{styles.card} {isHero ? styles.hero : styles.row}">
  {#if badge}
    <span class={styles.badge}>{badge}</span>
  {/if}

  <div class={styles.head}>
    <Avatar {persona} size={isHero ? 128 : 52} />
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
