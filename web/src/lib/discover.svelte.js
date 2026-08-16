// Discover state lives in a module, not in the component, so leaving the screen
// and coming back resumes on the same card of the same batch. A full page
// reload loses it, which the spec accepts.

import { api } from './api.js';
import { errorMessage } from './errors.js';

/** How many upcoming cards a just-passed persona stays hidden for. */
const PASS_COOLDOWN_CARDS = 5;

/** Prefetch once this few cards are left in the current batch. */
const PREFETCH_THRESHOLD = 2;

export const discover = $state({
  queue: [],
  loading: false,
  error: null,
  exhausted: false,
  /** [{ id, remaining }] — local only, never sent to the server as truth. */
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

/** Ages the cooldown list by one evaluated card. */
function tickCooldown() {
  discover.cooldown = discover.cooldown
    .map((entry) => ({ ...entry, remaining: entry.remaining - 1 }))
    .filter((entry) => entry.remaining > 0);
}

/** Drops the current card after it has been liked or passed. */
export function consumeCurrent({ cooldownId = null } = {}) {
  discover.queue = discover.queue.slice(1);
  tickCooldown();
  if (cooldownId) {
    discover.cooldown = [...discover.cooldown, { id: cooldownId, remaining: PASS_COOLDOWN_CARDS }];
  }
}

/** Removes a persona that the server says is no longer a valid target. */
export function dropFromQueue(personaId) {
  discover.queue = discover.queue.filter((p) => p.id !== personaId);
}

/**
 * Fetches the next batch and appends the personas that are not already queued,
 * so the user never sees a duplicate card or a visible batch boundary.
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

    const { personas } = await api.get(path);
    const known = new Set(discover.queue.map((p) => p.id));
    const fresh = personas.filter((p) => !known.has(p.id));

    discover.queue = [...discover.queue, ...fresh];
    discover.exhausted = discover.queue.length === 0;
  } catch (e) {
    discover.error = errorMessage(e);
  } finally {
    discover.loading = false;
  }
}

/** Keeps the queue topped up without the user noticing a batch boundary. */
export async function ensureCards() {
  if (discover.queue.length <= PREFETCH_THRESHOLD) {
    await fetchBatch();
  }
}
