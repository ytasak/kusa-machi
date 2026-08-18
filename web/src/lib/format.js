// 表示用のヘルパー。ここが返すのはすべてプレーンな文字列で、描画時に Svelte が
// エスケープする。HTML として挿入される値は一切ない。

const GENDER_LABELS = {
  male: '男性',
  female: '女性',
};

export function genderLabel(gender) {
  return GENDER_LABELS[gender] ?? gender;
}

export function ageAndGender(persona) {
  return `${persona.age}歳・${genderLabel(persona.gender)}`;
}

export function height(persona) {
  return `${persona.height_cm}cm`;
}

export function income(persona) {
  return `${persona.annual_income}万円`;
}

/**
 * ミリ秒の経過時間を HH:MM 形式にする。分は切り上げる。
 *
 * 「あと何分か」を出したいので、残り30秒を 00:00 とは出さない。ヘッダーには
 * 秒まで刻む日次カウントダウンが既にあり、そこに秒を持つ数字を2つ並べると
 * どちらを読むのか分からなくなるため、こちらは分までにする。
 */
export function minutesLeft(ms) {
  const total = Math.max(0, Math.ceil(ms / 60000));
  const hours = Math.floor(total / 60);
  const minutes = total % 60;
  return [hours, minutes].map((n) => String(n).padStart(2, '0')).join(':');
}

/** ミリ秒の経過時間を HH:MM:SS 形式にする。 */
export function countdown(ms) {
  const total = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  return [hours, minutes, seconds].map((n) => String(n).padStart(2, '0')).join(':');
}
