<script>
  import styles from './Modal.module.css';

  let { title, children } = $props();
</script>

<div class={styles.backdrop} role="dialog" aria-modal="true" aria-label={title}>
  <div class={styles.panel}>
    <h2 class={styles.title}>{title}</h2>
    {@render children?.()}
  </div>
</div>
