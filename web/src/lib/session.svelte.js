// ゲーム日のグローバル状態。当日の Persona、マイページに出すカウンタ、
// 日付境界を持つ。すべてサーバが正であり、このモジュールは直近の応答を
// 写しているだけ。あわせてカウントダウン用の時計オフセットを保持する。

import { api, ApiError, setCsrfToken } from './api.js';
import { resetDiscover } from './discover.svelte.js';
import { errorMessage } from './errors.js';

export const session = $state({
  loading: true,
  error: null,

  gameDate: null,
  personaGenerated: false,
  persona: null,

  remainingLikes: 10,
  receivedLikeCount: 0,
  matchCount: 0,
  hasUnseenLikes: false,
  hasUnseenMatches: false,

  /** ブラウザの時計に足すとサーバ時刻になるミリ秒。 */
  clockOffsetMs: 0,
  /** ゲーム日が終わり、新しい人生を始める必要がある状態になったら true。 */
  dayEnded: false,
});

/** ブラウザの時計ずれを補正したサーバ時刻（ミリ秒）。 */
export function serverNow() {
  return Date.now() + session.clockOffsetMs;
}

/** 現在のゲーム日が終わる瞬間。翌日の JST 00:00。 */
export function dayEndsAt() {
  if (!session.gameDate) return null;
  const start = new Date(`${session.gameDate}T00:00:00+09:00`).getTime();
  return start + 24 * 60 * 60 * 1000;
}

export function remainingMs() {
  return remainingMsFrom(Date.now());
}

/**
 * ブラウザのタイムスタンプを起点にしたゲーム日の残りミリ秒。
 * 各コンポーネントは共有ティッカーの値を渡すことで、カウントダウンを
 * リアクティブに保つ。
 */
export function remainingMsFrom(browserNow) {
  const end = dayEndsAt();
  return end === null ? 0 : end - (browserNow + session.clockOffsetMs);
}

function applyHome(home) {
  session.clockOffsetMs = new Date(home.server_time).getTime() - Date.now();

  const dayChanged = session.gameDate !== null && session.gameDate !== home.game_date;

  session.gameDate = home.game_date;
  session.personaGenerated = home.persona_generated;
  session.persona = home.persona;
  session.remainingLikes = home.remaining_likes;
  session.receivedLikeCount = home.received_like_count;
  session.matchCount = home.match_count;
  session.hasUnseenLikes = home.has_unseen_likes;
  session.hasUnseenMatches = home.has_unseen_matches;
  session.dayEnded = false;

  setCsrfToken(home.csrf_token);

  if (dayChanged) resetDiscover();
}

/** ホームのペイロードを取得する。同時に当日の participant の存在も保証される。 */
export async function refreshHome() {
  const home = await api.get('/api/home');
  applyHome(home);
  return home;
}

/** 初期ロード。エラーは投げずに画面へ表示する。 */
export async function bootstrap() {
  session.loading = true;
  session.error = null;
  try {
    await refreshHome();
  } catch (e) {
    session.error = errorMessage(e);
  } finally {
    session.loading = false;
  }
}

/** 「新しい人生を始める」を押す。サーバ側で冪等であり、振り直しは起きない。 */
export async function startNewLife() {
  // 先にホームを読み直す。日付が変わっていた場合に新しい CSRF トークンを得るため。
  await refreshHome();
  const persona = await api.post('/api/persona');
  session.persona = persona;
  session.personaGenerated = true;
  resetDiscover();
  return persona;
}

/** 縮小済みの JPEG Blob を当日の写真としてアップロードする。 */
export async function uploadPhoto(blob) {
  session.persona = await api.upload('/api/persona/photo', blob);
  return session.persona;
}

export async function deletePhoto() {
  session.persona = await api.delete('/api/persona/photo');
  return session.persona;
}

export async function updateProfile(fields) {
  const persona = await api.patch('/api/persona/profile', fields);
  session.persona = persona;
  return persona;
}

/** カウントダウンが0になったとき、または API が DayExpired を返したときに呼ぶ。 */
export function markDayEnded() {
  session.dayEnded = true;
}

/**
 * API 呼び出しをラップし、どのエンドポイントが最初に気づいたかによらず、
 * ゲーム日の終了が常に同じ画面に着地するようにする。
 */
export async function withDayGuard(fn) {
  try {
    return await fn();
  } catch (e) {
    if (e instanceof ApiError && e.code === 'DayExpired') {
      markDayEnded();
    }
    throw e;
  }
}
