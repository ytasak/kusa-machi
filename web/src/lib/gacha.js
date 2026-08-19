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

/** 開示する項目をすべて解決する。確定値・レア度・回転中の候補が1つに揃う。 */
export function revealItems(persona) {
  return REVEAL_STEPS.map((step) => {
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
