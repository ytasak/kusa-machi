<script>
  import ui from '../components/ui.module.css';
  import styles from './ProfileEdit.module.css';
  import PersonaCard from '../components/PersonaCard.svelte';
  import { session, updateProfile, withDayGuard } from '../lib/session.svelte.js';
  import { goHome } from '../lib/nav.svelte.js';

  const LIMITS = { name: 20, hobby: 30, bio: 60 };

  let name = $state(session.persona?.name ?? '');
  let hobby = $state(session.persona?.hobby ?? '');
  let bio = $state(session.persona?.bio ?? '');

  let saving = $state(false);
  let error = $state(null);
  let saved = $state(false);

  // The server counts characters, not bytes; [...s].length matches that.
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
      error = e.message;
    } finally {
      saving = false;
    }
  }
</script>

<section class={ui.screen}>
  <header class={ui.header}>
    <button class={ui.back} onclick={goHome}>← ホーム</button>
    <h1 class={ui.title}>プロフィール編集</h1>
  </header>

  {#if session.persona}
    <PersonaCard persona={session.persona} badge="あなた" />

    <form class={styles.form} onsubmit={save}>
      <div class={styles.field}>
        <div class={styles.labelRow}>
          <label class={styles.label} for="name">名前</label>
          <span class="{styles.counter} {lengths.name > LIMITS.name ? styles.counterOver : ''}">
            {lengths.name} / {LIMITS.name}
          </span>
        </div>
        <input class={styles.input} id="name" bind:value={name} maxlength={LIMITS.name} />
      </div>

      <div class={styles.field}>
        <div class={styles.labelRow}>
          <label class={styles.label} for="hobby">趣味</label>
          <span class="{styles.counter} {lengths.hobby > LIMITS.hobby ? styles.counterOver : ''}">
            {lengths.hobby} / {LIMITS.hobby}
          </span>
        </div>
        <input class={styles.input} id="hobby" bind:value={hobby} maxlength={LIMITS.hobby} />
      </div>

      <div class={styles.field}>
        <div class={styles.labelRow}>
          <label class={styles.label} for="bio">ひとこと</label>
          <span class="{styles.counter} {lengths.bio > LIMITS.bio ? styles.counterOver : ''}">
            {lengths.bio} / {LIMITS.bio}
          </span>
        </div>
        <input class={styles.input} id="bio" bind:value={bio} maxlength={LIMITS.bio} />
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
