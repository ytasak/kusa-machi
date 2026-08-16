// カウントダウンを表示するすべての箇所で共有する1秒ごとのティック。
// コンポーネントごとに interval を持たせないため。

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
