<script>
  // ブラウザが Cookie を保存してくれない場合の行き止まり画面。
  //
  // kusa の iframe の中ではこのアプリはサードパーティなので、古い iOS Safari
  // などでは Cookie が遮断される。その状態では毎回別人として扱われ、
  // 「新しい人生を始める」すら通らないため、遊べる場所へ誘導する。
  import styles from './CookieBlocked.module.css';
  import ui from '../components/ui.module.css';
  import Icon from '../components/Icon.svelte';

  // 埋め込み元ではなく、このアプリ自身を最上位で開く。
  const appUrl = typeof location === 'undefined' ? '/' : location.origin + '/';
</script>

<section class={styles.screen}>
  <div class={styles.mark}>
    <Icon name="person" size={36} />
  </div>

  <h1 class={styles.title}>この画面では遊べません</h1>

  <div class={styles.lead}>
    <p>お使いのブラウザが、埋め込み表示でのデータ保存を許可していません。</p>
    <p>別のタブで開くと、そのまま遊べます。</p>
  </div>

  <a class="{ui.primary} {styles.cta}" href={appUrl} target="_blank" rel="noopener noreferrer">
    別のタブで開く
  </a>
</section>
