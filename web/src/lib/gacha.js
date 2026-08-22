// 「新しい人生」を1属性ずつ開示する演出のためのデータ。
//
// ここが決めるのは見せ方だけ。抽選そのものはサーバの internal/persona が
// 済ませていて、この画面は確定済みの結果を順にめくって見せているにすぎない。
// レア度も演出のための後付けの解釈であり、マッチングの挙動には一切影響しない。

import { genderLabel } from './format.js';

/** レア度。数値がそのまま総合レア度のスコアになる。 */
export const TIER = { normal: 0, rare: 1, legend: 2 };

const normal = () => ({ tier: TIER.normal, note: null });
const rare = (note) => ({ tier: TIER.rare, note });
const legend = (note) => ({ tier: TIER.legend, note });

function randomInt(lo, hi) {
  return lo + Math.floor(Math.random() * (hi - lo + 1));
}

function sample(list) {
  return list[Math.floor(Math.random() * list.length)];
}

// スロットが回っているあいだに流す候補。サーバ側 internal/persona/generator.go の
// 一覧を写したもので、ずれても回転中の見た目が変わるだけ。確定値には影響しない。
const EDUCATIONS = ['中卒', '高卒', '専門卒', '短大卒', '大卒', '大学院卒', 'ホイ卒'];

const OCCUPATIONS = [
  '公務員',
  '医師',
  '看護師',
  '教員',
  'ITエンジニア',
  '営業',
  '事務',
  '販売・接客',
  '飲食',
  '建設',
  'クリエイター',
  '自営業',
  '経営者',
  'フリーター',
  '無職',
];

/**
 * 開示する順番。サーバが抽選する順（年齢→性別→身長→学歴→職業→年収）に
 * 揃えてあり、振れ幅がいちばん大きい年収が最後の山場に来る。
 *
 * chars は「この項目に出うる最長の文字数」。回転中に候補が入れ替わっても
 * 文字サイズが暴れないよう、ステップ単位で先に大きさを決めるために使う。
 */
export const REVEAL_STEPS = [
  {
    key: 'age',
    label: '年齢',
    chars: 3,
    value: (p) => `${p.age}歳`,
    spin: () => `${randomInt(20, 50)}歳`,
    rarity: (p) => {
      if (p.age <= 20) return rare('最年少');
      if (p.age >= 50) return rare('最年長');
      return normal();
    },
  },
  {
    key: 'gender',
    label: '性別',
    chars: 2,
    value: (p) => genderLabel(p.gender),
    spin: () => sample(['男性', '女性']),
    // 50/50 なので当たり外れは無い。テンポを作るための短い一拍。
    rarity: () => normal(),
  },
  {
    key: 'height',
    label: '身長',
    chars: 5,
    value: (p) => `${p.height_cm}cm`,
    spin: () => `${randomInt(140, 200)}cm`,
    rarity: (p) => {
      if (p.height_cm >= 198) return legend('規格外');
      if (p.height_cm <= 142) return legend('規格外');
      if (p.height_cm >= 190) return rare('高身長');
      if (p.height_cm <= 150) return rare('小柄');
      return normal();
    },
  },
  {
    key: 'education',
    label: '学歴',
    chars: 4,
    value: (p) => p.education,
    spin: () => sample(EDUCATIONS),
    rarity: (p) => {
      if (p.education === 'ホイ卒') return legend('伝説の学歴');
      if (p.education === '大学院卒') return rare('高学歴');
      if (p.education === '中卒') return rare('叩き上げ');
      return normal();
    },
  },
  {
    key: 'occupation',
    label: '職業',
    chars: 7,
    value: (p) => p.occupation,
    spin: () => sample(OCCUPATIONS),
    rarity: (p) => {
      if (p.occupation === '医師') return legend('エリート');
      if (p.occupation === '経営者') return legend('成功者');
      if (p.occupation === '無職') return legend('ネタ枠');
      if (p.occupation === 'フリーター') return rare('自由人');
      return normal();
    },
  },
  {
    key: 'income',
    label: '年収',
    chars: 7,
    value: (p) => `${p.annual_income}万円`,
    spin: () => `${randomInt(0, 180) * 10}万円`,
    rarity: (p) => {
      if (p.annual_income >= 1000) return legend('大台突破');
      if (p.annual_income >= 700) return rare('高収入');
      if (p.annual_income <= 100) return rare('崖っぷち');
      return normal();
    },
  },
];

/**
 * 項目ごとの回転時間。後半ほど長く溜める。REVEAL_STEPS と同じ並び。
 * 回転から確定までの運びは GachaReveal が持つので、長さだけをここに置く。
 */
export const REVEAL_SPIN_MS = [420, 320, 500, 640, 780, 1000];

/**
 * 子ガチャの回転時間。CHILD_REVEAL_STEPS と同じ並び。
 *
 * 人生ガチャより短くする。人生は1日1回きりの山場だが、子ガチャは Match の
 * たびに引けるので、同じ長さで待たせると重い。
 */
export const CHILD_REVEAL_SPIN_MS = [300, 380, 460, 560, 700];

/**
 * 年齢を持たない対象の開示項目。子ガチャがこれを使う。
 *
 * 判定そのものは REVEAL_STEPS から1つ外すだけで、子ガチャ専用のしきい値も
 * 専用の評価体系も持たない。身長・学歴・職業・年収のレア度は、他人の人生を
 * 見るときとまったく同じ基準で読む。
 */
export const CHILD_REVEAL_STEPS = REVEAL_STEPS.filter((step) => step.key !== 'age');

/**
 * 開示する項目をすべて解決する。確定値・レア度・回転中の候補が1つに揃う。
 *
 * steps を差し替えられるのは、年齢を持たない子ガチャのため。属性が欠けている
 * 対象に対して、その属性のステップごと外して同じ判定を通す。
 */
export function revealItems(persona, steps = REVEAL_STEPS) {
  return steps.map((step) => {
    const { tier, note } = step.rarity(persona);
    return {
      key: step.key,
      label: step.label,
      chars: step.chars,
      value: step.value(persona),
      spin: step.spin,
      tier,
      note,
    };
  });
}

// 総合レア度。レア度の合計で決まる。期待値はおよそ1なので、
// たいていは N か R に落ち、SR 以上はたまにしか出ない。
const RANKS = [
  { min: 5, label: 'SSR', copy: '伝説の人生を引き当てました', tier: TIER.legend },
  { min: 3, label: 'SR', copy: 'なかなかの人生です', tier: TIER.legend },
  { min: 1, label: 'R', copy: '少し変わった人生です', tier: TIER.rare },
  { min: 0, label: 'N', copy: 'ごく普通の人生です', tier: TIER.normal },
];

export function rankOf(items) {
  const score = items.reduce((sum, item) => sum + item.tier, 0);
  return { ...RANKS.find((r) => score >= r.min), score };
}

/**
 * Persona 1件の総合レア度。
 *
 * レア度の入口はこの関数だけにする。開示演出・探す・一覧のどこから呼んでも
 * 同じ Persona なら同じ答えになる。A属性は生成時に確定してその日は動かないので、
 * ここも何度呼んでも結果は変わらない。
 */
export function personaRank(persona) {
  return rankOf(revealItems(persona));
}

/** 子ガチャの開示項目。年齢のステップだけを外した同じ判定を通す。 */
export function childRevealItems(child) {
  return revealItems(child, CHILD_REVEAL_STEPS);
}

/**
 * 子1人の総合レア度。
 *
 * personaRank と同じ理由で、子のレア度の入口もこの関数だけにする。属性は
 * 生成時に保存されて動かないので、Match 詳細を開き直しても同じレア度になる。
 * 親のレア度は入力に含まれない。SSR どうしから N も出るし、その逆も出る。
 */
export function childRank(child) {
  return rankOf(childRevealItems(child));
}
