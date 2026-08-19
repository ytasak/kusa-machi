<script>
  import { onMount } from 'svelte';
  import styles from './Discover.module.css';
  import ui from '../components/ui.module.css';
  import Icon from '../components/Icon.svelte';
  import PersonaCard from '../components/PersonaCard.svelte';
  import MatchAnimation from '../components/MatchAnimation.svelte';
  import { api, ApiError } from '../lib/api.js';
  import { session, withDayGuard, applyLikeState } from '../lib/session.svelte.js';
  import { discover, currentCard, consumeCurrent, ensureCards } from '../lib/discover.svelte.js';
  import { go, SCREENS } from '../lib/nav.svelte.js';
  import { errorMessage } from '../lib/errors.js';
  import { vibrate, HAPTICS } from '../lib/haptics.js';
  import { personaRank } from '../lib/gacha.js';

  // 送り出したカードが画面外へ抜けるまでの時間。CSS の flyLike / flyPass と揃える。
  const EXIT_MS = 460;

  /** 光条・粒・レア度の判子まで出すレア度。ここから下は静かに出す。 */
  const LOUD_RANKS = new Set(['SR', 'SSR']);

  /**
   * 弾ける粒。散り方は先に決めておき、カードが変わるたびに引き直さない。
   * 中心から放射状に散らし、3粒ごとに1粒だけ遠くまで飛ばす。
   * SR は前半10粒だけを使う（残りは CSS で隠す）。
   *
   * dx / dy は飛ぶ向きだけを持つ無次元の値。実際の飛距離は CSS 側の --spread が
   * 画面幅を見て決める。こうしておけば、狭い画面でも粒がカードの外へ出ない。
   */
  const BURST = Array.from({ length: 14 }, (_, i) => {
    const angle = (i / 14) * Math.PI * 2 + (i % 2 ? 0.22 : 0);
    const reach = i % 3 === 0 ? 1 : 0.74;
    return {
      shape: ['✦', '★', '✧'][i % 3],
      dx: (Math.cos(angle) * reach).toFixed(3),
      dy: (Math.sin(angle) * reach).toFixed(3),
      scale: i % 3 === 0 ? 1.2 : 0.85,
      rotate: -150 + i * 24,
      delay: (i % 4) * 26,
      duration: 620 + (i % 3) * 90,
    };
  });

  let message = $state(null);
  /** 送り出し中のカード: { persona, kind, seq } */
  let exiting = $state(null);
  let matchedPersona = $state(null);
  /** 直近の Match で回復した Like 数。演出の中に出す。 */
  let matchedLikesGained = $state(0);
  /**
   * レア度の演出を鳴らし直すための通し番号。1枚評価するたびに進む。
   *
   * Persona の id だけを鍵にすると、その日の候補が少ないときに同じ相手が
   * 続けて出た瞬間だけ鍵が変わらず、演出が飛んでしまう。評価した回数で
   * 数えれば、何が出ても必ず鳴る。
   *
   * Match 演出を閉じたときにも進める。Like を送った時点で次のカードはもう
   * 出ているので、Match 演出のあいだにその裏で演出が終わってしまう。閉じた
   * ところで鳴らし直して「評価 -> 次の人生 -> レア度」の並びを崩さない。
   */
  let revealSeq = $state(0);

  const card = $derived(currentCard());
  const outOfLikes = $derived(session.remainingLikes <= 0);

  // 次に出るカードのレア度。人生ガチャと同じ lib/gacha.js の判定をそのまま使う。
  // A属性から決まるので、ここで計算し直しても同じ Persona なら常に同じ答えになる。
  const rank = $derived(card ? personaRank(card) : null);
  /**
   * 演出の再発火キー。カードが変わるたびに値が変わるので、同じレア度でも
   * 同じ相手でも、出るたびに演出が最初から走る。
   */
  const revealKey = $derived(`${revealSeq}-${card?.id ?? ''}`);
  const loud = $derived(LOUD_RANKS.has(rank?.label));

  // 出た瞬間の手応え。SR 以上だけ一段強くして、目を離していても希少さが伝わる。
  // revealKey を読んでいるので、鳴るタイミングは演出と同じになる。
  $effect(() => {
    const label = revealKey && rank?.label;
    if (label === 'SSR') vibrate(HAPTICS.rank);
    else if (label === 'SR') vibrate(HAPTICS.revealRare);
  });

  onMount(refill);

  /**
   * カードを補充する。探索の応答には残数と次回回復も入っているので、
   * この往復が時間回復を画面へ反映する機会にもなる。
   */
  async function refill() {
    const res = await ensureCards();
    if (res) applyLikeState(res);
  }

  // 「このカードはもう古い」という意味でしかないエラー。カードはすでに
  // 送り出しているので、ユーザーに見せるべきことは何も無い。
  const STALE_CODES = new Set(['AlreadyLiked', 'TargetPersonaUnavailable']);

  let exitTimer = null;
  let throwSeq = 0;

  /**
   * 判定したカードを次のカードの上に重ねてから飛ばす。
   *
   * seq を鍵にして毎回別の要素として描画するので、連打しても
   * アニメーションが最初から再生され、押した回数ぶん手応えが返る。
   */
  function throwCard(persona, kind) {
    exiting = { persona, kind, seq: ++throwSeq };
    // 判定したこの瞬間が、次のカードが出る瞬間でもある。演出の鍵をここで進める。
    revealSeq += 1;
    clearTimeout(exitTimer);
    exitTimer = setTimeout(() => {
      exiting = null;
    }, EXIT_MS);
  }

  /**
   * Like と Pass は応答を待たずに画面を進める。
   *
   * サーバは常に正であり、成功時のレスポンスで残数を確定させる。往復のあいだ
   * 指を止めさせないためにこうしている。1往復ぶん（本番で約80ms）の待ちが
   * 体感から消える。
   */
  async function onLike() {
    const target = card;
    if (!target || outOfLikes) return;

    message = null;

    // 楽観更新。ここで残数を減らすので、連打しても上限を超えて送れない。
    session.remainingLikes -= 1;
    consumeCurrent();
    throwCard(target, 'like');
    vibrate(HAPTICS.like);
    refill();

    try {
      const res = await withDayGuard(() => api.post('/api/likes', { persona_id: target.id }));

      // サーバの値で確定させ、楽観更新のズレをここで吸収する。
      applyLikeState(res);

      if (res.matched) {
        session.matchCount += 1;
        matchedPersona = res.target_persona;
        matchedLikesGained = res.likes_gained;
        vibrate(HAPTICS.match);
      }
    } catch (e) {
      revertLike(e);
    }
  }

  // 失敗した Like は消費されていないので残数を戻す。ズレても次の成功時に
  // サーバの値で上書きされる。
  function revertLike(e) {
    if (e instanceof ApiError && e.code === 'LikeLimitExceeded') {
      session.remainingLikes = 0;
    } else {
      session.remainingLikes = Math.min(session.likeCapacity, session.remainingLikes + 1);
    }

    if (!(e instanceof ApiError) || !STALE_CODES.has(e.code)) {
      message = errorMessage(e);
    }
  }

  async function onPass() {
    const target = card;
    if (!target) return;

    message = null;

    // Pass は消費する予算が無いので、送り出したら結果を待つ必要がない。
    // サーバは Pass で候補を減らさないので、再表示を遅らせるのはこのローカルの
    // クールダウンだけ。だから Pass のたびに必ず付ける。
    consumeCurrent({ cooldownId: target.id });
    throwCard(target, 'pass');
    vibrate(HAPTICS.pass);
    refill();

    try {
      await withDayGuard(() => api.post('/api/passes', { persona_id: target.id }));
    } catch (e) {
      if (!(e instanceof ApiError) || !STALE_CODES.has(e.code)) {
        message = errorMessage(e);
      }
    }
  }
</script>

<section class={styles.screen}>
  <div class={styles.topRow}>
    <h1 class={ui.title}>探す</h1>
    <span class={styles.receivedBadge}>
      <Icon name="heart" size={13} filled />
      Likeされた {session.receivedLikeCount}
    </span>
  </div>

  {#if message}
    <p class={ui.error}>{message}</p>
  {/if}

  {#if card || exiting}
    <div class={styles.stage}>
      <div class={styles.cardBox}>
        {#if card}
          <!-- key を付けることで、カードが変わるたびに出現とレア度の演出が走る。
               data-rank が演出の強さと長さを決め、SSR でも 0.8 秒で終わる。
               重ねているだけなので、演出中も Like / Pass はそのまま押せる。 -->
          {#key revealKey}
            <div class={styles.cardLayer} data-rank={rank.label}>
              <span class={styles.revealRays} aria-hidden="true"></span>
              <PersonaCard persona={card} variant="hero" rarityReveal />
              <span class={styles.revealGlow} aria-hidden="true"></span>
              <span class={styles.revealFlash} aria-hidden="true"></span>

              <!-- SR 以上だけ、人生ガチャと同じ「粒が弾けてレア度が判子で押される」
                   出し方にする。どちらも一瞬で消えるので操作の邪魔にはならない。 -->
              {#if loud}
                <span class={styles.burst} aria-hidden="true">
                  {#each BURST as p, i (i)}
                    <span
                      class={styles.particle}
                      style="--dx:{p.dx}; --dy:{p.dy}; --sc:{p.scale}; --rot:{p.rotate}deg;
                             --delay:{p.delay}ms; --dur:{p.duration}ms"
                    >
                      {p.shape}
                    </span>
                  {/each}
                </span>
                <span class={styles.revealStamp} aria-hidden="true">{rank.label}</span>
              {/if}
            </div>
          {/key}
        {/if}

        {#if exiting}
          {#key exiting.seq}
            <div
              class="{styles.exitLayer} {exiting.kind === 'like' ? styles.exitLike : styles.exitPass}"
            >
              <PersonaCard persona={exiting.persona} variant="hero" />
              <span
                class="{styles.tint} {exiting.kind === 'like' ? styles.tintLike : styles.tintPass}"
              ></span>
              <span
                class="{styles.stamp} {exiting.kind === 'like' ? styles.stampLike : styles.stampPass}"
              >
                {exiting.kind === 'like' ? 'LIKE' : 'PASS'}
              </span>
            </div>
          {/key}
        {/if}
      </div>
    </div>

    <div class={ui.actions}>
      <button class="{ui.circle} {ui.circlePass}" onclick={onPass} aria-label="パス">
        <Icon name="close" size={26} />
      </button>
      <button
        class="{ui.circle} {ui.circleLike}"
        onclick={onLike}
        disabled={outOfLikes}
        aria-label="Like"
      >
        <Icon name="heart" size={30} filled />
      </button>
    </div>

    <!-- Like が尽きた瞬間がいちばん「どうすれば増えるのか」を知りたいところ。
         まだ取れる報酬があればそれを出し、無ければ Match だけを案内する。 -->
    {#if outOfLikes}
      {#if session.profileRewardAvailable}
        <button class={styles.recovery} onclick={() => go(SCREENS.profile)}>
          Likeを使い切りました。プロフィールを完成させると<strong>+1</strong>戻ります
        </button>
      {:else}
        <p class={styles.recoveryNote}>
          Likeを使い切りました。Matchが決まると<strong>+2</strong>戻ります（1日2回まで）
        </p>
      {/if}
    {/if}
  {:else if discover.loading}
    <p class={ui.empty}>読み込み中...</p>
  {:else if discover.error}
    <p class={ui.error}>{discover.error}</p>
  {:else}
    <p class={ui.empty}>いま出会える人がいません。<br />しばらくしてからまた見てみてください。</p>
  {/if}
</section>

{#if matchedPersona}
  <MatchAnimation
    own={session.persona}
    counterpart={matchedPersona}
    likesGained={matchedLikesGained}
    onclose={() => {
      matchedPersona = null;
      revealSeq += 1;
    }}
  />
{/if}
