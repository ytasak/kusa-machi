// Screen navigation.
//
// The four tab screens are always one tap away; 送信済みLike and
// プロフィール編集 are pushed screens that live under マイページ.

export const SCREENS = {
  discover: 'discover',
  receivedLikes: 'receivedLikes',
  matches: 'matches',
  mypage: 'mypage',
  sentLikes: 'sentLikes',
  profile: 'profile',
};

/** The tab bar, in the spec's navigation priority order. */
export const TABS = [
  { screen: SCREENS.discover, label: '探す', icon: 'search' },
  { screen: SCREENS.receivedLikes, label: 'Likeされた', icon: 'heart' },
  { screen: SCREENS.matches, label: 'Match', icon: 'sparkle' },
  { screen: SCREENS.mypage, label: 'マイページ', icon: 'person' },
];

/** Pushed screens map back to the tab they belong to. */
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

/** Which tab should look selected for the current screen. */
export function activeTab(screen) {
  return PARENT_TAB[screen] ?? screen;
}
