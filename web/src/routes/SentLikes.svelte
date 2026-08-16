<script>
  import { onMount } from 'svelte';
  import ui from '../components/ui.module.css';
  import PersonaCard from '../components/PersonaCard.svelte';
  import { api } from '../lib/api.js';
  import { goHome } from '../lib/nav.svelte.js';
  import { errorMessage } from '../lib/errors.js';

  let personas = $state([]);
  let loading = $state(true);
  let error = $state(null);

  onMount(async () => {
    try {
      const res = await api.get('/api/likes/sent');
      personas = res.personas;
    } catch (e) {
      error = errorMessage(e);
    } finally {
      loading = false;
    }
  });
</script>

<section class={ui.screen}>
  <header class={ui.header}>
    <button class={ui.back} onclick={goHome}>← ホーム</button>
    <h1 class={ui.title}>送信済みLike</h1>
  </header>

  {#if loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if error}
    <p class={ui.error}>{error}</p>
  {:else if personas.length === 0}
    <p class={ui.empty}>まだ誰にもLikeを送っていません。</p>
  {:else}
    <ul class={ui.list}>
      {#each personas as persona (persona.id)}
        <li>
          <PersonaCard {persona} badge={persona.matched ? 'MATCH' : null} />
        </li>
      {/each}
    </ul>
  {/if}
</section>
