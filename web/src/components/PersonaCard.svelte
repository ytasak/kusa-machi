<script>
  import styles from './PersonaCard.module.css';
  import Avatar from './Avatar.svelte';
  import RarityBadge from './RarityBadge.svelte';
  import { ageAndGender, height, income } from '../lib/format.js';
  import { personaRank } from '../lib/gacha.js';

  // 探す / Likeされた / 送信済みLike / Match で共通のカード。
  // variant が変えるのは情報密度だけで、属性の並びは常に仕様どおり:
  // 名前、年齢+性別、身長、職業、年収、学歴、趣味、ひとこと。
  // 未設定の項目は丸ごと省く。
  //
  // rarityReveal はカードが切り替わった瞬間の強調を鳴らすかどうか。探す画面の
  // 現在のカードだけ true にする。一覧では静止したバッジで足りる。
  let { persona, badge = null, variant = 'row', rarityReveal = false } = $props();

  const isHero = $derived(variant === 'hero');
  // レア度は A属性から決まるので、同じ Persona ならどの画面でも同じ答えになる。
  // 判定は人生ガチャと同じ lib/gacha.js のもので、ここでは計算し直さない。
  const rank = $derived(personaRank(persona));
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
      <p class={styles.headline}>
        <!-- hero では CSS でカード左上へ出す。row では年齢の頭に並べて、
             行の高さを増やさずにプロフィールと同時に目に入るようにする。 -->
        <span class={styles.rarity}>
          <RarityBadge {rank} size={isHero ? 'lg' : 'sm'} reveal={rarityReveal} />
        </span>
        {ageAndGender(persona)}
      </p>
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
