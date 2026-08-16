<script>
  import styles from './TabBar.module.css';
  import Icon from './Icon.svelte';
  import { nav, go, activeTab, TABS, SCREENS } from '../lib/nav.svelte.js';
  import { session } from '../lib/session.svelte.js';

  const current = $derived(activeTab(nav.screen));

  function hasDot(screen) {
    if (screen === SCREENS.receivedLikes) return session.hasUnseenLikes;
    if (screen === SCREENS.matches) return session.hasUnseenMatches;
    return false;
  }
</script>

<nav class={styles.bar}>
  {#each TABS as tab (tab.screen)}
    {@const isActive = current === tab.screen}
    <button
      class="{styles.tab} {isActive ? styles.active : ''}"
      aria-current={isActive ? 'page' : undefined}
      onclick={() => go(tab.screen)}
    >
      <Icon name={tab.icon} size={22} filled={isActive && tab.icon !== 'search' && tab.icon !== 'person'} />
      <span>{tab.label}</span>
      {#if hasDot(tab.screen)}
        <span class={styles.dot}></span>
      {/if}
    </button>
  {/each}
</nav>
