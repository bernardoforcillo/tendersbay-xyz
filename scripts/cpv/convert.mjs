// Converts the European Commission's CPV 2008 vocabulary from its published
// wide form (one row per code, one column per language) into the long form the
// ingestion seeder reads: code,lang,label — gzipped.
//
// The header is DISCOVERED, not assumed. The published file's column order has
// changed between revisions, and hard-coding it would silently mislabel every
// row rather than fail. Anything that is not a 2-letter language column or the
// code column is ignored.
//
// Usage:
//   node scripts/cpv/convert.mjs <input.csv> <output.csv.gz>
//
// The input must be a comma-separated UTF-8 export of the official
// spreadsheet's main-vocabulary sheet, first row = header.

import { readFileSync, writeFileSync } from 'node:fs';
import { gzipSync } from 'node:zlib';

/** The 24 official EU language codes, lowercase ISO 639-1. */
const EU_LANGS = new Set([
  'bg',
  'cs',
  'da',
  'de',
  'el',
  'en',
  'es',
  'et',
  'fi',
  'fr',
  'ga',
  'hr',
  'hu',
  'it',
  'lt',
  'lv',
  'mt',
  'nl',
  'pl',
  'pt',
  'ro',
  'sk',
  'sl',
  'sv',
]);

/**
 * Parses a whole CSV document into rows of fields, tracking quote state across
 * the entire character stream rather than pre-splitting on newlines first.
 *
 * That distinction matters: a quoted cell is allowed to contain a literal
 * newline (a genuinely multi-line label), and splitting into "lines" with a
 * regex before any quote parsing — as an earlier version of this script did —
 * cuts such a field in two. The tail fragment's first column then fails
 * `normaliseCode` and gets silently discarded as a "blank row" by the
 * `if (!code) continue` guard below, dropping every remaining language for
 * that code with no error and no trace in the output. Reading char-by-char
 * means a newline inside quotes is just another character, not a row
 * boundary.
 */
function parseCsv(text) {
  const rows = [];
  let row = [];
  let field = '';
  let quoted = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (quoted) {
      // Every character is literal while quoted, including \r and \n — that's
      // exactly what lets an embedded newline survive intact.
      if (ch === '"' && text[i + 1] === '"') {
        field += '"';
        i++;
      } else if (ch === '"') quoted = false;
      else field += ch;
      continue;
    }
    if (ch === '"') quoted = true;
    else if (ch === ',') {
      row.push(field);
      field = '';
    } else if (ch === '\r') {
      // Swallow bare CR; a following LF (or its absence) ends the row below.
    } else if (ch === '\n') {
      row.push(field);
      field = '';
      rows.push(row);
      row = [];
    } else field += ch;
  }
  if (field !== '' || row.length > 0) {
    row.push(field);
    rows.push(row);
  }
  // Drop wholly blank lines (a single empty field), matching the old
  // line-filter behaviour.
  return rows.filter((r) => !(r.length === 1 && r[0] === ''));
}

/** Strips the optional check digit: "03000000-1" -> "03000000". */
function normaliseCode(raw) {
  const digits = raw.trim().split('-')[0].replace(/\D/g, '');
  return digits.length === 8 ? digits : null;
}

/**
 * Codes with a known, deliberate gap in the official export — a genuine hole
 * in the European Commission's data, not something this converter dropped.
 * The value is how many of the 24 language labels that code is missing. Any
 * code short of a label that isn't listed here (or short by a different
 * amount than listed) fails the build: an unexplained gap means labels were
 * silently lost — e.g. by the embedded-newline bug `parseCsv` exists to
 * avoid — rather than genuinely absent from the source. Updating this map is
 * a deliberate, reviewed decision, not a workaround.
 */
const KNOWN_GAPS = new Map([
  // Missing Hungarian and Italian labels in every CPV 2008 export checked so
  // far — confirmed present in exactly 22 of the 24 languages.
  ['03117140', 2],
]);

function main() {
  const [input, output] = process.argv.slice(2);
  if (!input || !output) {
    console.error('usage: node scripts/cpv/convert.mjs <input.csv> <output.csv.gz>');
    process.exit(2);
  }

  const allRows = parseCsv(readFileSync(input, 'utf8'));
  if (allRows.length < 2) throw new Error(`${input}: expected a header plus data rows`);

  const header = allRows[0].map((h) => h.trim().toLowerCase());

  // The code column is whichever header looks like a code; everything else that
  // is a 2-letter EU language becomes a label column.
  const codeIdx = header.findIndex((h) => h === 'code' || h === 'cpv' || h === 'cpv code');
  if (codeIdx < 0) throw new Error(`${input}: no code column in header: ${header.join('|')}`);

  const langCols = [];
  header.forEach((h, i) => {
    if (i !== codeIdx && EU_LANGS.has(h)) langCols.push({ lang: h, index: i });
  });
  if (langCols.length !== 24) {
    throw new Error(
      `${input}: found ${langCols.length} EU language columns (${langCols.map((c) => c.lang).join(',')}), want 24`,
    );
  }

  const rows = [];
  const codes = new Set();
  const labelCount = new Map();
  for (let i = 1; i < allRows.length; i++) {
    const cells = allRows[i];
    const code = normaliseCode(cells[codeIdx] ?? '');
    if (!code) continue; // section headings and blank rows
    codes.add(code);
    for (const { lang, index } of langCols) {
      const label = (cells[index] ?? '').trim();
      if (!label) continue;
      labelCount.set(code, (labelCount.get(code) ?? 0) + 1);
      // Quote every label: they contain commas freely.
      rows.push(`${code},${lang},"${label.replace(/"/g, '""')}"`);
    }
  }

  if (codes.size < 9000) {
    throw new Error(
      `${input}: only ${codes.size} distinct codes; CPV 2008 has ~9450 — the export is incomplete`,
    );
  }

  // Every code should carry a label in every discovered language, with the
  // sole exception named in KNOWN_GAPS above. Checking this here — not just
  // trusting the row count — is what makes a silent partial loss (rather than
  // a genuine source gap) fail the build instead of shipping quietly.
  for (const code of codes) {
    const have = labelCount.get(code) ?? 0;
    const want = langCols.length - (KNOWN_GAPS.get(code) ?? 0);
    if (have !== want) {
      throw new Error(
        `${input}: code ${code} has ${have}/${langCols.length} labels, want ${want} — an unexplained gap means labels were silently dropped rather than genuinely absent from the source (update KNOWN_GAPS if this is a deliberate, verified change)`,
      );
    }
  }

  writeFileSync(output, gzipSync(Buffer.from(`code,lang,label\n${rows.join('\n')}\n`, 'utf8')));
  console.log(`wrote ${output}: ${codes.size} codes × up to 24 languages = ${rows.length} rows`);
}

main();
