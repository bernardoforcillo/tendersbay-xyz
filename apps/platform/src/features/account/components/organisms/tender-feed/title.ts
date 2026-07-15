import { countryAlpha2 } from './country';

/**
 * Notice titles arrive with the country spelled out as a lead segment, in the
 * notice's own language and set off by a spaced dash or a colon — e.g.
 * "Italia – Apparecchi per angiografia – Affidamento della fornitura …". The
 * card already shows the origin as a flag, so that country prefix is redundant.
 * `tenderTitle` drops it and lets the title lead with the actual object.
 *
 * The prefix is removed *only* when the lead segment is genuinely this tender's
 * country name (matched across the EU languages, accent-insensitively), so any
 * other dash in the title — including a legitimate leading clause — is kept.
 */

// The 24 official EU languages: a notice's country name is spelled in one of
// them, so this is the set we match the lead segment against.
const EU_LANGUAGES = [
  'en',
  'it',
  'fr',
  'de',
  'es',
  'pt',
  'nl',
  'pl',
  'sv',
  'da',
  'fi',
  'el',
  'cs',
  'hu',
  'ro',
  'bg',
  'hr',
  'sk',
  'sl',
  'et',
  'lv',
  'lt',
  'ga',
  'mt',
];

const nameCache = new Map<string, Set<string>>();

/** lower-cased, accent-stripped form for language-agnostic comparison. */
function normalize(value: string): string {
  return value
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLowerCase()
    .trim();
}

/** Every EU-language spelling of a country, normalised, memoised per country. */
function countryNames(code: string): Set<string> {
  const alpha2 = countryAlpha2(code);
  if (!alpha2) return new Set();
  const cached = nameCache.get(alpha2);
  if (cached) return cached;

  const names = new Set<string>();
  for (const language of EU_LANGUAGES) {
    try {
      const name = new Intl.DisplayNames([language], { type: 'region' }).of(alpha2);
      if (name && name !== alpha2) names.add(normalize(name));
    } catch {
      // Language not supported by the runtime's Intl data — skip it.
    }
  }
  nameCache.set(alpha2, names);
  return names;
}

// Splits a title into its lead segment and the remainder at the first spaced
// dash (– — -) or colon separator. Non-greedy, so it stops at the first one.
const LEAD_SEGMENT = /^(.+?)(?:\s+[–—-]|:)\s+(.+)$/u;

/** The tender title with its redundant leading country name removed. */
export function tenderTitle(rawTitle: string, country: string): string {
  const title = rawTitle.trim();
  const match = LEAD_SEGMENT.exec(title);
  const lead = match?.[1];
  const rest = match?.[2];
  if (lead === undefined || rest === undefined) return title;

  if (countryNames(country).has(normalize(lead))) {
    return rest.trim() || title;
  }
  return title;
}
