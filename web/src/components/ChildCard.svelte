<script>
  import styles from './ChildCard.module.css';
  import RarityBadge from './RarityBadge.svelte';
  import { genderLabel, height, income } from '../lib/format.js';
  import { childRank } from '../lib/gacha.js';

  // 子ガチャの結果カード。
  //
  // Persona カードとは別の部品にしている。子は年齢も名前も趣味も持たず、
  // 写真も無いので、PersonaCard に無理に通すと空の行だけが増える。
  //
  // 見せているのは「子供本人の今のプロフィール」ではなく、2人から生まれた
  // 架空の将来ステータス。遺伝の予測ではなく、あくまでガチャの結果。
  let { child } = $props();

  // レア度は人生ガチャと同じ lib/gacha.js の判定を、年齢のステップだけ外して
  // 通したもの。属性はサーバに保存済みなので、開き直しても同じ答えになる。
  const rank = $derived(childRank(child));
</script>

<article class={styles.card}>
  <span class={styles.rarity}>
    <RarityBadge {rank} size="lg" />
  </span>

  <p class={styles.headline}>{genderLabel(child.gender)}</p>

  <div class={styles.tags}>
    <span class={styles.tag}><span class={styles.tagLabel}>身長</span>{height(child)}</span>
    <span class={styles.tag}><span class={styles.tagLabel}>職業</span>{child.occupation}</span>
    <span class={styles.tag}><span class={styles.tagLabel}>年収</span>{income(child)}</span>
    <span class={styles.tag}><span class={styles.tagLabel}>学歴</span>{child.education}</span>
  </div>
</article>
