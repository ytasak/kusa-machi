// Discover の状態はコンポーネントではなくモジュールに置く。画面を離れて戻っても
// 同じバッチの同じカードから再開できるようにするため。ページを完全に再読み込み
// すると失われるが、それは仕様が許容している。

import { api } from './api.js';
import { errorMessage } from './errors.js';

/** Pass した直後の Persona を、この先何枚のカードのあいだ表示しないか。 */
const PASS_COOLDOWN_CARDS = 5;

/** 現在のバッチの残りがこの枚数を切ったらプリフェッチする。 */
const PREFETCH_THRESHOLD = 2;

export const discover = $state({
  queue: [],
  loading: false,
  error: null,
  exhausted: false,
  /** [{ id, remaining }] — ローカル専用。サーバに正として送ることはない。 */
  cooldown: [],
});

export function resetDiscover() {
  discover.queue = [];
  discover.loading = false;
  discover.error = null;
  discover.exhausted = false;
  discover.cooldown = [];
}

export function currentCard() {
  return discover.queue[0] ?? null;
}

function cooldownIds() {
  return discover.cooldown.map((entry) => entry.id);
}

/** 評価済みカード1枚ぶん、クールダウン一覧を進める。 */
function tickCooldown() {
  discover.cooldown = discover.cooldown
    .map((entry) => ({ ...entry, remaining: entry.remaining - 1 }))
    .filter((entry) => entry.remaining > 0);
}

/** Like か Pass の後に現在のカードを取り除く。 */
export function consumeCurrent({ cooldownId = null } = {}) {
  discover.queue = discover.queue.slice(1);
  tickCooldown();
  if (cooldownId) {
    discover.cooldown = [...discover.cooldown, { id: cooldownId, remaining: PASS_COOLDOWN_CARDS }];
  }
}

/**
 * 次のバッチを取得し、まだキューに無い Persona だけを追加する。
 * これにより同じカードが二度出ることも、バッチの切れ目が見えることもない。
 */
export async function fetchBatch() {
  if (discover.loading) return;

  discover.loading = true;
  discover.error = null;
  try {
    const exclude = [...cooldownIds(), ...discover.queue.map((p) => p.id)];
    const path = exclude.length > 0
      ? `/api/discover?exclude=${encodeURIComponent(exclude.join(','))}`
      : '/api/discover';

    const res = await api.get(path);
    const known = new Set(discover.queue.map((p) => p.id));
    const fresh = res.personas.filter((p) => !known.has(p.id));

    discover.queue = [...discover.queue, ...fresh];
    discover.exhausted = discover.queue.length === 0;
    // 応答には残数と次回回復も入っている。session を直接触ると
    // session -> discover の import と循環するので、呼び出し側に渡す。
    return res;
  } catch (e) {
    discover.error = errorMessage(e);
    return null;
  } finally {
    discover.loading = false;
  }
}

/**
 * ユーザーにバッチの切れ目を意識させずにキューを補充し続ける。
 * 取得した場合はその応答を返す。補充が不要だった場合は null。
 */
export async function ensureCards() {
  if (discover.queue.length <= PREFETCH_THRESHOLD) {
    return await fetchBatch();
  }
  return null;
}
