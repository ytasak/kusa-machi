// Display helpers. Everything here returns plain strings; Svelte escapes them
// on render, and no value is ever inserted as HTML.

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

/** Formats a duration in milliseconds as HH:MM:SS. */
export function countdown(ms) {
  const total = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  return [hours, minutes, seconds].map((n) => String(n).padStart(2, '0')).join(':');
}
