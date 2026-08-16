<script>
  import { onMount } from 'svelte';
  import ui from '../components/ui.module.css';
  import PersonaCard from '../components/PersonaCard.svelte';
  import { api } from '../lib/api.js';
  import { session } from '../lib/session.svelte.js';
  import { goHome } from '../lib/nav.svelte.js';

  let personas = $state([]);
  let loading = $state(true);
  let error = $state(null);

  onMount(async () => {
    try {
      const res = await api.get('/api/matches');
      personas = res.personas;
      // Opening the screen is what clears the badge server-side.
      session.hasUnseenMatches = false;
      session.matchCount = personas.length;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });
</script>

<section class={ui.screen}>
  <header class={ui.header}>
    <button class={ui.back} onclick={goHome}>← ホーム</button>
    <h1 class={ui.title}>Match</h1>
  </header>

  {#if loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if error}
    <p class={ui.error}>{error}</p>
  {:else if personas.length === 0}
    <p class={ui.empty}>まだマッチしていません。</p>
  {:else}
    <ul class={ui.list}>
      {#each personas as persona (persona.id)}
        <li>
          <PersonaCard {persona} badge="MATCH" />
        </li>
      {/each}
    </ul>
  {/if}
</section>
