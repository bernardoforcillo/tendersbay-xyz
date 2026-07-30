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
 * Splits one CSV line, honouring double-quoted fields and "" escapes. Written
 * out rather than pulled from a dependency: this script runs once per
 * vocabulary revision and a new package would have to clear the repo's
 * 7-day minimumReleaseAge gate for no ongoing benefit.
 */
function splitCsvLine(line) {
  const out = [];
  let field = '';
  let quoted = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (quoted) {
      if (ch === '"' && line[i + 1] === '"') {
        field += '"';
        i++;
      } else if (ch === '"') quoted = false;
      else field += ch;
    } else if (ch === '"') quoted = true;
    else if (ch === ',') {
      out.push(field);
      field = '';
    } else field += ch;
  }
  out.push(field);
  return out;
}

/** Strips the optional check digit: "03000000-1" -> "03000000". */
function normaliseCode(raw) {
  const digits = raw.trim().split('-')[0].replace(/\D/g, '');
  return digits.length === 8 ? digits : null;
}

function main() {
  const [input, output] = process.argv.slice(2);
  if (!input || !output) {
    console.error('usage: node scripts/cpv/convert.mjs <input.csv> <output.csv.gz>');
    process.exit(2);
  }

  const lines = readFileSync(input, 'utf8')
    .split(/\r?\n/)
    .filter((l) => l.length > 0);
  if (lines.length < 2) throw new Error(`${input}: expected a header plus data rows`);

  const header = splitCsvLine(lines[0]).map((h) => h.trim().toLowerCase());

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
  for (let i = 1; i < lines.length; i++) {
    const cells = splitCsvLine(lines[i]);
    const code = normaliseCode(cells[codeIdx] ?? '');
    if (!code) continue; // section headings and blank rows
    codes.add(code);
    for (const { lang, index } of langCols) {
      const label = (cells[index] ?? '').trim();
      if (!label) continue;
      // Quote every label: they contain commas freely.
      rows.push(`${code},${lang},"${label.replace(/"/g, '""')}"`);
    }
  }

  if (codes.size < 9000) {
    throw new Error(
      `${input}: only ${codes.size} distinct codes; CPV 2008 has ~9450 — the export is incomplete`,
    );
  }

  writeFileSync(output, gzipSync(Buffer.from(`code,lang,label\n${rows.join('\n')}\n`, 'utf8')));
  console.log(`wrote ${output}: ${codes.size} codes × up to 24 languages = ${rows.length} rows`);
}

main();
