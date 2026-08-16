<script>
  import ui from '../components/ui.module.css';
  import styles from './ProfileEdit.module.css';
  import Icon from '../components/Icon.svelte';
  import PersonaCard from '../components/PersonaCard.svelte';
  import Avatar from '../components/Avatar.svelte';
  import { session, updateProfile, uploadPhoto, deletePhoto, withDayGuard } from '../lib/session.svelte.js';
  import { goMyPage } from '../lib/nav.svelte.js';
  import { errorMessage } from '../lib/errors.js';
  import { prepareUpload } from '../lib/image.js';

  const LIMITS = { name: 20, hobby: 30, bio: 60 };

  let name = $state(session.persona?.name ?? '');
  let hobby = $state(session.persona?.hobby ?? '');
  let bio = $state(session.persona?.bio ?? '');

  let saving = $state(false);
  let error = $state(null);
  let saved = $state(false);

  let fileInput;
  let photoBusy = $state(false);
  let photoError = $state(null);

  async function onPickPhoto(event) {
    const file = event.target.files?.[0];
    // 失敗した後でも同じファイルを選び直せるようにする。
    event.target.value = '';
    if (!file) return;

    photoBusy = true;
    photoError = null;
    try {
      // ここでの縮小はアップロードを小さくするためだけ。サーバが再エンコードする。
      const blob = await prepareUpload(file);
      await withDayGuard(() => uploadPhoto(blob));
    } catch (e) {
      photoError = e.name === 'ApiError' ? errorMessage(e) : '画像を読み込めませんでした。';
    } finally {
      photoBusy = false;
    }
  }

  async function onRemovePhoto() {
    photoBusy = true;
    photoError = null;
    try {
      await withDayGuard(() => deletePhoto());
    } catch (e) {
      photoError = errorMessage(e);
    } finally {
      photoBusy = false;
    }
  }

  // サーバはバイトではなく文字数で数える。[...s].length がその数え方に一致する。
  const lengths = $derived({
    name: [...name].length,
    hobby: [...hobby].length,
    bio: [...bio].length,
  });

  const tooLong = $derived(
    lengths.name > LIMITS.name || lengths.hobby > LIMITS.hobby || lengths.bio > LIMITS.bio,
  );

  async function save(event) {
    event.preventDefault();
    saving = true;
    error = null;
    saved = false;
    try {
      await withDayGuard(() => updateProfile({ name, hobby, bio }));
      name = session.persona.name ?? '';
      hobby = session.persona.hobby ?? '';
      bio = session.persona.bio ?? '';
      saved = true;
    } catch (e) {
      error = errorMessage(e);
    } finally {
      saving = false;
    }
  }
</script>

<section class={ui.screen}>
  <div class={ui.header}>
    <button class={ui.back} onclick={goMyPage} aria-label="マイページに戻る">
      <Icon name="back" size={18} />
    </button>
    <h1 class={ui.title}>プロフィール編集</h1>
  </div>

  {#if session.persona}
    <PersonaCard persona={session.persona} variant="hero" badge="あなた" />
    <p class={styles.hint}>
      年齢・性別・身長・職業・年収・学歴は今日のあなたの設定です。変更はできません。
    </p>

    <div class={styles.photoBlock}>
      <Avatar persona={session.persona} size={84} />
      <div class={styles.photoActions}>
        <button class={ui.secondary} onclick={() => fileInput.click()} disabled={photoBusy}>
          {session.persona.photo_url ? '写真を変更' : '写真を選ぶ'}
        </button>
        {#if session.persona.photo_url}
          <button class={styles.remove} onclick={onRemovePhoto} disabled={photoBusy}>削除</button>
        {/if}
        <p class={styles.hint}>正方形に切り抜いて保存します。写真も0時に消えます。</p>
      </div>
      <input
        class={styles.file}
        type="file"
        accept="image/jpeg,image/png,image/*"
        bind:this={fileInput}
        onchange={onPickPhoto}
      />
    </div>

    {#if photoError}<p class={ui.error}>{photoError}</p>{/if}

    <form class={styles.form} onsubmit={save}>
      <div class={styles.field}>
        <div class={styles.labelRow}>
          <label class={styles.label} for="name">名前</label>
          <span class="{styles.counter} {lengths.name > LIMITS.name ? styles.counterOver : ''}">
            {lengths.name} / {LIMITS.name}
          </span>
        </div>
        <input class={styles.input} id="name" bind:value={name} maxlength={LIMITS.name} placeholder="呼ばれたい名前" />
      </div>

      <div class={styles.field}>
        <div class={styles.labelRow}>
          <label class={styles.label} for="hobby">趣味</label>
          <span class="{styles.counter} {lengths.hobby > LIMITS.hobby ? styles.counterOver : ''}">
            {lengths.hobby} / {LIMITS.hobby}
          </span>
        </div>
        <input class={styles.input} id="hobby" bind:value={hobby} maxlength={LIMITS.hobby} placeholder="休日にしていること" />
      </div>

      <div class={styles.field}>
        <div class={styles.labelRow}>
          <label class={styles.label} for="bio">ひとこと</label>
          <span class="{styles.counter} {lengths.bio > LIMITS.bio ? styles.counterOver : ''}">
            {lengths.bio} / {LIMITS.bio}
          </span>
        </div>
        <input class={styles.input} id="bio" bind:value={bio} maxlength={LIMITS.bio} placeholder="ひとことどうぞ" />
      </div>

      <p class={styles.hint}>すべて任意です。空欄にするとカードに表示されません。URLは登録できません。</p>

      {#if error}<p class={ui.error}>{error}</p>{/if}
      {#if saved}<p class={styles.saved}>保存しました</p>{/if}

      <button class={ui.primary} type="submit" disabled={saving || tooLong}>保存する</button>
    </form>
  {:else}
    <p class={ui.empty}>先に今日の人生を始めてください。</p>
  {/if}
</section>
