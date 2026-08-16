// Screen navigation. Home is the hub; there is no persistent tab bar, so a
// single current-screen value is all the app needs.

export const SCREENS = {
  home: 'home',
  discover: 'discover',
  receivedLikes: 'receivedLikes',
  sentLikes: 'sentLikes',
  matches: 'matches',
  profile: 'profile',
};

export const nav = $state({ screen: SCREENS.home });

export function go(screen) {
  nav.screen = screen;
}

export function goHome() {
  nav.screen = SCREENS.home;
}
