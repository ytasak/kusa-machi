<script>
  import { onMount } from 'svelte';
  import ui from '../components/ui.module.css';
  import styles from './Matches.module.css';
  import Icon from '../components/Icon.svelte';
  import PersonaCard from '../components/PersonaCard.svelte';
  import { api } from '../lib/api.js';
  import { session } from '../lib/session.svelte.js';
  import { goMatchDetail } from '../lib/nav.svelte.js';
  import { errorMessage } from '../lib/errors.js';

  let matches = $state([]);
  let loading = $state(true);
  let error = $state(null);

  onMount(async () => {
    try {
      const res = await api.get('/api/matches');
      matches = res.personas;
      // サーバ側でバッジが消えるのは、この画面を開いたとき。
      session.hasUnseenMatches = false;
      session.matchCount = matches.length;
    } catch (e) {
      error = errorMessage(e);
    } finally {
      loading = false;
    }
  });
</script>

<section class={ui.screen}>
  <div class={ui.header}>
    <h1 class={ui.title}>Match</h1>
  </div>

  {#if loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if error}
    <p class={ui.error}>{error}</p>
  {:else if matches.length === 0}
    <p class={ui.empty}>まだマッチしていません。<br />Likeを送ってみましょう。</p>
  {:else}
    <ul class={ui.list}>
      {#each matches as match (match.match_id)}
        <li>
          <!-- カードごとタップして Match 詳細へ。子ガチャを引いていなければ
               そこで引けるし、引いていればその子をもう一度見られる。 -->
          <button class={styles.card} onclick={() => goMatchDetail(match.match_id)}>
            <PersonaCard persona={match} badge="MATCH" />
            <span class={styles.footer}>
              <span class={match.child_generated ? styles.hasChild : styles.prompt}>
                {match.child_generated ? '👶 子あり' : 'この2人の子を引く'}
              </span>
              <Icon name="chevron" size={16} />
            </span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</section>
