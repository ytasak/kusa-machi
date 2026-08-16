// Global game-day state: today's persona, the counters shown on Home, and the
// day boundary. The server is authoritative for all of it; this module only
// mirrors the last answer and tracks the clock offset for the countdown.

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

  /** Milliseconds to add to the browser clock to get server time. */
  clockOffsetMs: 0,
  /** Set once the game day is over and the user must start a new life. */
  dayEnded: false,
});

/** Server time as a millisecond timestamp, corrected for browser clock skew. */
export function serverNow() {
  return Date.now() + session.clockOffsetMs;
}

/** The instant the current game day ends: 00:00 JST of the following day. */
export function dayEndsAt() {
  if (!session.gameDate) return null;
  const start = new Date(`${session.gameDate}T00:00:00+09:00`).getTime();
  return start + 24 * 60 * 60 * 1000;
}

export function remainingMs() {
  return remainingMsFrom(Date.now());
}

/**
 * Milliseconds left in the game day, measured from a browser timestamp.
 * Components pass the shared ticker value so the countdown stays reactive.
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

/** Fetches the home payload, which also guarantees today's participant exists. */
export async function refreshHome() {
  const home = await api.get('/api/home');
  applyHome(home);
  return home;
}

/** Initial load. Errors are surfaced on screen rather than thrown. */
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

/** Presses 新しい人生を始める. Idempotent server-side; never rerolls. */
export async function startNewLife() {
  // Re-read home first so a day that rolled over gets a fresh CSRF token.
  await refreshHome();
  const persona = await api.post('/api/persona');
  session.persona = persona;
  session.personaGenerated = true;
  resetDiscover();
  return persona;
}

/** Uploads an already-resized JPEG blob as today's picture. */
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

/** Called when the countdown reaches zero, or an API call reports DayExpired. */
export function markDayEnded() {
  session.dayEnded = true;
}

/**
 * Wraps an API call so an expired game day always lands on the same screen,
 * whichever endpoint noticed it first.
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
