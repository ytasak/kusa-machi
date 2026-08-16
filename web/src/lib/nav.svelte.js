// 画面遷移。
//
// 4つのタブ画面へはいつでも1タップで移動できる。送信済みLike と
// プロフィール編集は マイページ の配下にプッシュされる画面。

export const SCREENS = {
  discover: 'discover',
  receivedLikes: 'receivedLikes',
  matches: 'matches',
  mypage: 'mypage',
  sentLikes: 'sentLikes',
  profile: 'profile',
};

/** タブバー。仕様のナビゲーション優先順で並べる。 */
export const TABS = [
  { screen: SCREENS.discover, label: '探す', icon: 'search' },
  { screen: SCREENS.receivedLikes, label: 'Likeされた', icon: 'heart' },
  { screen: SCREENS.matches, label: 'Match', icon: 'sparkle' },
  { screen: SCREENS.mypage, label: 'マイページ', icon: 'person' },
];

/** プッシュされた画面は、所属するタブに対応付ける。 */
const PARENT_TAB = {
  [SCREENS.sentLikes]: SCREENS.mypage,
  [SCREENS.profile]: SCREENS.mypage,
};

export const nav = $state({ screen: SCREENS.discover });

export function go(screen) {
  nav.screen = screen;
}

export function goMyPage() {
  nav.screen = SCREENS.mypage;
}

/** 現在の画面に対して、どのタブを選択状態に見せるか。 */
export function activeTab(screen) {
  return PARENT_TAB[screen] ?? screen;
}
