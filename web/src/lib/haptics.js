// 触覚フィードバック。
//
// navigator.vibrate は Android の Chrome では効くが、iOS Safari は非対応。
// 対応していない環境では単に何も起きない、あくまで上乗せの演出として扱う。

/** 操作ごとの振動パターン。数値はミリ秒、配列は 振動/停止 の繰り返し。 */
export const HAPTICS = {
  like: 18,
  pass: 10,
  match: [0, 45, 60, 45, 60, 120],
  start: [0, 30, 40, 70],
};

export function vibrate(pattern) {
  if (typeof navigator === 'undefined' || typeof navigator.vibrate !== 'function') return;
  try {
    navigator.vibrate(pattern);
  } catch {
    // 端末やブラウザ設定で拒否されることがある。演出なので黙って諦める。
  }
}
