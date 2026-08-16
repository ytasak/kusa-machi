// 新しい人生の開示演出の状態。
//
// 開始画面と「今日の人生が終了しました」モーダルの2か所から人生は始まるが、
// どちらから入っても同じ演出に着地させたいので、ここに集約する。
//
// 演出をアプリ全体に被せるオーバーレイにしているのには理由がある。
// startNewLife() が解決した時点で session.personaGenerated が true になり、
// App.svelte は開始画面をアンマウントしてタブ UI に切り替わる。画面の中に
// 演出を置くと、結果を出す前に消えてしまう。

import { startNewLife } from './session.svelte.js';
import { errorMessage } from './errors.js';
import { go, SCREENS } from './nav.svelte.js';

/** 抽選を待たせる最短時間。即答されても溜めが無いとガチャにならない。 */
const MIN_ROLL_MS = 1100;

export const reveal = $state({
  /** オーバーレイを出しているあいだ true。 */
  active: false,
  /** 抽選結果。届くまでは null で、そのあいだは溜めの演出を出す。 */
  persona: null,
  /** 抽選に失敗したときのメッセージ。 */
  error: null,
});

/**
 * 新しい人生を引く。オーバーレイは即座に開き、結果が届いてから開示が始まる。
 * サーバ側は冪等なので、失敗して引き直しても振り直しにはならない。
 */
export async function rollNewLife() {
  reveal.active = true;
  reveal.persona = null;
  reveal.error = null;

  const startedAt = Date.now();
  try {
    const persona = await startNewLife();
    const elapsed = Date.now() - startedAt;
    if (elapsed < MIN_ROLL_MS) {
      await new Promise((resolve) => setTimeout(resolve, MIN_ROLL_MS - elapsed));
    }
    // 演出を閉じたときにマイページへ着地するよう、裏側だけ先に移動しておく。
    go(SCREENS.mypage);
    reveal.persona = persona;
  } catch (e) {
    reveal.error = errorMessage(e);
  }
}

export function closeReveal() {
  reveal.active = false;
  reveal.persona = null;
  reveal.error = null;
}
