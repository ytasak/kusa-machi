// 画面遷移。
//
// 5つのタブ画面へはいつでも1タップで移動できる。プロフィール編集と Match 詳細が
// タブの配下にプッシュされる画面。

export const SCREENS = {
  discover: 'discover',
  receivedLikes: 'receivedLikes',
  matches: 'matches',
  mypage: 'mypage',
  sentLikes: 'sentLikes',
  profile: 'profile',
  matchDetail: 'matchDetail',
};

/**
 * 下部ナビの主 CTA。「探す」はこのアプリの主行動なので、タブの1つとして
 * 端に並べず、中央に独立した1つのボタンとして置く。
 */
export const PRIMARY_TAB = { screen: SCREENS.discover, label: '探す', icon: 'search' };

/**
 * 主 CTA の左右に2つずつ並ぶタブ。この順で左から並べる。
 * 中央に近いほど優先度が高い（Likeされた / Match）。
 */
export const SIDE_TABS = [
  { screen: SCREENS.sentLikes, label: '送信済み', icon: 'send' },
  { screen: SCREENS.receivedLikes, label: 'Likeされた', icon: 'heart', fillWhenActive: true },
  { screen: SCREENS.matches, label: 'Match', icon: 'sparkle', fillWhenActive: true },
  { screen: SCREENS.mypage, label: 'マイページ', icon: 'person' },
];

/** プッシュされた画面は、所属するタブに対応付ける。 */
const PARENT_TAB = {
  [SCREENS.profile]: SCREENS.mypage,
  [SCREENS.matchDetail]: SCREENS.matches,
};

export const nav = $state({
  screen: SCREENS.discover,
  /** Match 詳細で開いている match_id。他の画面では使わない。 */
  matchID: null,
  /**
   * 開いた直後に子ガチャを始めるか。Match 成立演出の「この2人の子を引く」から
   * 入ったときだけ true になり、詳細画面が読み取ったところで下ろす。
   */
  matchAutoDraw: false,
});

export function go(screen) {
  nav.screen = screen;
}

/**
 * Match 詳細を開く。
 *
 * draw を立てると、開いた直後に子ガチャが始まる。Match 成立演出から
 * 「この2人の子を引く」で入る経路がこれで、一覧のカードからは false で入る。
 */
export function goMatchDetail(matchID, { draw = false } = {}) {
  nav.matchID = matchID;
  nav.matchAutoDraw = draw;
  nav.screen = SCREENS.matchDetail;
}

/** Match 詳細から一覧へ戻る。 */
export function goMatches() {
  nav.matchID = null;
  nav.matchAutoDraw = false;
  nav.screen = SCREENS.matches;
}

export function goMyPage() {
  nav.screen = SCREENS.mypage;
}

/** 現在の画面に対して、どのタブを選択状態に見せるか。 */
export function activeTab(screen) {
  return PARENT_TAB[screen] ?? screen;
}
