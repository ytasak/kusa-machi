// A single one-second tick shared by everything that displays the countdown,
// so the app never runs one interval per component.

export const ticker = $state({ now: Date.now() });

let handle = null;

export function startTicker() {
  if (handle !== null) return;
  handle = setInterval(() => {
    ticker.now = Date.now();
  }, 1000);
}

export function stopTicker() {
  if (handle === null) return;
  clearInterval(handle);
  handle = null;
}
