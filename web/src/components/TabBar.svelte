<script>
  import styles from './TabBar.module.css';
  import Icon from './Icon.svelte';
  import { nav, go, activeTab, PRIMARY_TAB, SIDE_TABS, SCREENS } from '../lib/nav.svelte.js';
  import { session } from '../lib/session.svelte.js';

  // 「探す」は主行動なので、タブの1つではなく中央のボタンとして置く。
  // 左右にタブが2つずつ並び、中央に近いほど優先度が高い。
  const left = SIDE_TABS.slice(0, 2);
  const right = SIDE_TABS.slice(2);

  const current = $derived(activeTab(nav.screen));
  const primaryActive = $derived(current === PRIMARY_TAB.screen);

  function hasDot(screen) {
    if (screen === SCREENS.receivedLikes) return session.hasUnseenLikes;
    if (screen === SCREENS.matches) return session.hasUnseenMatches;
    return false;
  }
</script>

{#snippet sideTab(tab)}
  {@const isActive = current === tab.screen}
  <button
    class="{styles.tab} {isActive ? styles.active : ''}"
    aria-current={isActive ? 'page' : undefined}
    onclick={() => go(tab.screen)}
  >
    <Icon name={tab.icon} size={22} filled={isActive && tab.fillWhenActive} />
    <span>{tab.label}</span>
    {#if hasDot(tab.screen)}
      <span class={styles.dot}></span>
    {/if}
  </button>
{/snippet}

<nav class={styles.bar}>
  {#each left as tab (tab.screen)}
    {@render sideTab(tab)}
  {/each}

  <button
    class="{styles.primary} {primaryActive ? styles.primaryActive : ''}"
    aria-current={primaryActive ? 'page' : undefined}
    onclick={() => go(PRIMARY_TAB.screen)}
  >
    <span class={styles.primaryIcon}>
      <Icon name={PRIMARY_TAB.icon} size={26} />
    </span>
    <span class={styles.primaryLabel}>{PRIMARY_TAB.label}</span>
  </button>

  {#each right as tab (tab.screen)}
    {@render sideTab(tab)}
  {/each}
</nav>
