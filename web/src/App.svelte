<script>
  import { onMount } from 'svelte';
  import { api } from './lib/api.js';
  import styles from './App.module.css';

  let status = $state('接続中...');
  let error = $state(null);

  onMount(async () => {
    try {
      const res = await api.get('/api/health');
      status = `API: ${res.status}`;
    } catch (e) {
      error = e.message;
    }
  });
</script>

<main class={styles.shell}>
  <h1 class={styles.title}>今日の人生</h1>
  {#if error}
    <p class={styles.error}>{error}</p>
  {:else}
    <p class={styles.status}>{status}</p>
  {/if}
</main>
