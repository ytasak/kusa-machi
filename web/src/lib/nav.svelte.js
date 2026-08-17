// 画面遷移。
//
// 5つのタブ画面へはいつでも1タップで移動できる。プロフィール編集だけが
// マイページの配下にプッシュされる画面。

export const SCREENS = {
  discover: 'discover',
  receivedLikes: 'receivedLikes',
  matches: 'matches',
  mypage: 'mypage',
  sentLikes: 'sentLikes',
  profile: 'profile',
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
