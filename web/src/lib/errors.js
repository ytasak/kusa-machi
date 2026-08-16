// The API answers with a stable error.code; the wording shown to the user is
// decided here so the UI never surfaces a raw server message.

import { ApiError } from './api.js';

const MESSAGES = {
  PersonaNotGenerated: '先に今日の人生を始めてください。',
  LikeLimitExceeded: '今日のLikeを使い切りました。',
  AlreadyLiked: 'この人にはすでにLikeを送っています。',
  TargetPersonaUnavailable: 'この人は今日の市場にいません。',
  PassLimitReached: 'この人は今日はもう表示されません。',
  SelfActionNotAllowed: '自分には操作できません。',
  DayExpired: '今日の人生は終了しました。',
  InvalidProfileInput: '改行とURLは登録できません。文字数もご確認ください。',
  InvalidCSRFToken: '通信が無効になりました。画面を再読み込みしてください。',
  InvalidRequest: 'リクエストが正しくありません。',
};

const FALLBACK = '通信に失敗しました。しばらくしてからお試しください。';

export function errorMessage(error) {
  if (error instanceof ApiError) {
    return MESSAGES[error.code] ?? FALLBACK;
  }
  return FALLBACK;
}
