import { countryAlpha2 } from './country';

/**
 * Notice titles arrive as "Country – Category – Object" in the notice's own
 * language — e.g. "Italia – Apparecchi per angiografia – Affidamento della
 * fornitura …". The card shows origin as a flag, so the country prefix is
 * redundant; the category is a classification best shown as a subtitle. So
 * `tenderTitle` returns the two useful parts:
 *
 * - `title`   — the object of the contract (the notice's real headline),
 * - `category` — the subject classification, or null when the notice has none.
 *
 * The country prefix is only stripped when the lead segment is genuinely this
 * tender's country name (matched across the EU languages, accent-insensitively),
 * and the category is only split off once that structured prefix is confirmed —
 * so an ordinary title with a stray dash or colon is returned whole, no
 * category, nothing mangled.
 */

export type TenderTitle = {
  title: string;
  category: string | null;
};

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

// The country prefix is set off by a spaced dash (– — -) or a colon; the
// category from the object by a spaced dash. Both non-greedy — first separator
// wins.
const COUNTRY_PREFIX = /^(.+?)(?:\s+[–—-]|:)\s+(.+)$/u;
const CATEGORY_SPLIT = /^(.+?)\s+[–—-]\s+(.+)$/u;

/** Splits a tender's raw title into its object headline and category subtitle. */
export function tenderTitle(rawTitle: string, country: string): TenderTitle {
  const title = rawTitle.trim();

  // Strip the leading country name — but only if the lead segment really is
  // this tender's country. Otherwise the title has no structured prefix.
  const prefix = COUNTRY_PREFIX.exec(title);
  const lead = prefix?.[1];
  const afterCountry = prefix?.[2]?.trim();
  if (lead === undefined || !afterCountry || !countryNames(country).has(normalize(lead))) {
    return { title, category: null };
  }

  // What's left is "Category – Object" (or just the object). Pull the leading
  // category out as the subtitle, keeping any further dashes with the object.
  const split = CATEGORY_SPLIT.exec(afterCountry);
  const category = split?.[1]?.trim();
  const object = split?.[2]?.trim();
  if (!category || !object) return { title: afterCountry, category: null };

  return { title: object, category };
}
