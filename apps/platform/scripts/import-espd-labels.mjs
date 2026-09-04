/**
 * Writes the `espd` namespace into all 24 locale files.
 *
 * Two sources, deliberately kept apart:
 *
 *  - **This product's sentences** come from `espd-copy.mjs` — everything the
 *    app says around the document (readiness, gaps, export, confirmations).
 *  - **The names of the exclusion grounds, the selection criteria and the Part
 *    III headings** come from the Union's own authority tables
 *    (`ExclusionGround.gc`, `SelectionCriterion.gc` in OP-TED/espd-edm,
 *    © European Union, EUPL-1.2), read here at import time.
 *
 * The second half is the point: "Grave professional misconduct" and "Gravi
 * illeciti professionali" are the same question, and an operator should not
 * have to work that out. Using the Union's wording means the phrase in this app
 * is the phrase on the contracting authority's form, in all 24 languages, and
 * it is the phrase their lawyer already knows.
 *
 * Run:
 *
 *   git clone --depth 1 https://github.com/OP-TED/espd-edm /tmp/espd-edm
 *   node scripts/import-espd-labels.mjs --edm /tmp/espd-edm
 *   pnpm exec biome check --write src/assets/locales
 *
 * It rewrites `src/assets/locales/<locale>/common.json` in place, replacing the
 * whole `espd` node. Everything else in the file is left untouched (the array
 * re-wrapping JSON.stringify does is undone by the biome pass above).
 */
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { COPY } from './espd-copy.mjs';
import { OFFICIAL_CODES, OFFICIAL_PART_CODES, OWN_CRITERIA } from './espd-official-labels.mjs';

const HERE = dirname(fileURLToPath(import.meta.url));
const LOCALES_DIR = join(HERE, '..', 'src', 'assets', 'locales');

/**
 * Our locale tags to the ISO 639-2 column the code lists carry. The tables are
 * keyed by language only — there is one official Portuguese, not a pt-PT and a
 * pt-BR — so the country half of our tag has no counterpart here.
 */
const LANG_COLUMN = {
  'bg-bg': 'bul',
  'cs-cz': 'ces',
  'da-dk': 'dan',
  'de-de': 'deu',
  'el-gr': 'ell',
  'en-ie': 'eng',
  'es-es': 'spa',
  'et-ee': 'est',
  'fi-fi': 'fin',
  'fr-fr': 'fra',
  'ga-ie': 'gle',
  'hr-hr': 'hrv',
  'hu-hu': 'hun',
  'it-it': 'ita',
  'lt-lt': 'lit',
  'lv-lv': 'lav',
  'mt-mt': 'mlt',
  'nl-nl': 'nld',
  'pl-pl': 'pol',
  'pt-pt': 'por',
  'ro-ro': 'ron',
  'sk-sk': 'slk',
  'sl-si': 'slv',
  'sv-se': 'swe',
};

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  return i >= 0 ? process.argv[i + 1] : fallback;
}

/**
 * Reads a genericode table into `code → { <lang column>: label }`.
 *
 * A hand-rolled reader rather than an XML dependency: these files are machine
 * generated, one `<Value ColumnRef>`/`<SimpleValue>` pair per cell, and the
 * script runs once per code-list release. Entities are decoded because the
 * labels carry `&amp;` and typographic apostrophes.
 */
function readCodeList(path) {
  const xml = readFileSync(path, 'utf8');
  const table = {};
  for (const row of xml.split('<Row>').slice(1)) {
    const cells = {};
    const re = /<Value ColumnRef="([^"]+)">\s*<SimpleValue>([\s\S]*?)<\/SimpleValue>/g;
    for (let m = re.exec(row); m !== null; m = re.exec(row)) {
      cells[m[1]] = decode(m[2]);
    }
    const code = cells.code;
    if (code) table[code] = cells;
  }
  return table;
}

function decode(s) {
  return s
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&apos;/g, "'")
    .replace(/&#(\d+);/g, (_, d) => String.fromCodePoint(Number(d)))
    .replace(/&amp;/g, '&')
    .trim();
}

/** `{'a.b': 'x'}` → `{a: {b: 'x'}}`. i18next reads `.` as its key separator. */
function nest(flat) {
  const out = {};
  for (const [path, value] of Object.entries(flat)) {
    const parts = path.split('.');
    let node = out;
    for (const part of parts.slice(0, -1)) {
      if (typeof node[part] !== 'object' || node[part] === null) node[part] = {};
      node = node[part];
    }
    node[parts.at(-1)] = value;
  }
  return out;
}

/** The line every locale file carries verbatim; the block goes in above it. */
const ANCHOR = '  "landing": {';

/**
 * Unwraps a `JSON.stringify({ espd: … }, null, 2)` result into just its member
 * lines, which are already indented for the outer object they are spliced into.
 */
function member(json) {
  return json.split('\n').slice(1, -1).join('\n');
}

/** Removes an existing top-level node, so the script is re-runnable. */
function drop(raw, key) {
  const start = raw.indexOf(`\n  "${key}": {`) + 1;
  const end = raw.indexOf('\n  },\n', start) + '\n  },\n'.length;
  return raw.slice(0, start) + raw.slice(end);
}

const edm = arg('edm', '/tmp/espd-edm');
const tables = {
  ...readCodeList(join(edm, 'codelists', 'ExclusionGround.gc')),
  ...readCodeList(join(edm, 'codelists', 'SelectionCriterion.gc')),
};

// Fail on a code the tables no longer carry rather than quietly writing the raw
// key into 24 files: a criterion renamed upstream is a mapping to fix, not a
// label to invent.
const problems = [];
for (const [key, code] of Object.entries({ ...OFFICIAL_CODES, ...OFFICIAL_PART_CODES })) {
  const row = tables[code];
  if (!row) {
    problems.push(`${key}: no row for code "${code}"`);
    continue;
  }
  for (const [locale, lang] of Object.entries(LANG_COLUMN)) {
    if (!row[`${lang}_label`]) problems.push(`${key} (${code}): no ${lang} label for ${locale}`);
  }
}
for (const own of OWN_CRITERIA) {
  for (const locale of Object.keys(LANG_COLUMN)) {
    if (!COPY[locale]?.[`criteria.${own}`]) problems.push(`${own}: no own copy for ${locale}`);
  }
}
if (problems.length > 0) {
  console.error(`${problems.length} problem(s):`);
  for (const p of problems.slice(0, 20)) console.error(`  ${p}`);
  process.exit(1);
}

let written = 0;
for (const [locale, lang] of Object.entries(LANG_COLUMN)) {
  const flat = { ...COPY[locale] };
  for (const [key, code] of Object.entries(OFFICIAL_CODES)) {
    flat[`criteria.${key}`] = tables[code][`${lang}_label`];
  }
  for (const [part, code] of Object.entries(OFFICIAL_PART_CODES)) {
    flat[`parts.${part}`] = tables[code][`${lang}_label`];
  }

  const path = join(LOCALES_DIR, locale, 'common.json');
  const raw = readFileSync(path, 'utf8');
  // Spliced as text, not re-serialised: JSON.parse/stringify would expand every
  // short object in the file and bury this change in a few hundred lines of
  // reformatting. The anchor exists verbatim in all 24 files, and `espd` lands
  // next to `company` — the two halves of the dossier.
  const block = `${member(JSON.stringify({ espd: nest(flat) }, null, 2))},\n`;
  const stripped = raw.includes(`\n  "espd": {`) ? drop(raw, 'espd') : raw;
  if (!stripped.includes(ANCHOR)) throw new Error(`${locale}: anchor not found`);
  writeFileSync(path, stripped.replace(ANCHOR, block + ANCHOR), 'utf8');

  written += 1;
}

const keys =
  Object.keys(COPY['en-ie']).length +
  Object.keys(OFFICIAL_CODES).length +
  Object.keys(OFFICIAL_PART_CODES).length;
console.log(`wrote espd (${keys} keys) into ${written} locales`);
