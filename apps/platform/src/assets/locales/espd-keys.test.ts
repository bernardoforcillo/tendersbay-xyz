import { describe, expect, it } from 'vitest';
import {
  DECLARATION_PARTS,
  ESPD_CRITERIA,
  ESPD_FORMATS,
  ESPD_VERSIONS,
  EXCLUSION_CRITERIA,
  GAP_REASONS,
  partOfCriterion,
} from '~/features/espd';

type LocaleModule = { default: Record<string, unknown> };
const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  LocaleModule
>;
const entries = Object.entries(modules);

function get(obj: unknown, path: string): unknown {
  return path
    .split('.')
    .reduce<unknown>((acc, key) => (acc as Record<string, unknown> | undefined)?.[key], obj);
}

// The DGUE is the one screen where an untranslated key is not a cosmetic bug.
// A person signs this document and is criminally liable for what it says, so a
// row reading `iii.c.grave_professional_misconduct` is a question they cannot
// answer and must not answer anyway. The criterion vocabularies in
// `~/features/espd` are 1:1 with the Go constants, so a ground added there
// without copy fails here in all 24 locales at once.
//
// The criterion NAMES these walk are not this project's translations: they are
// the Union's own, imported from the ESPD-EDM code lists by
// `scripts/import-espd-labels.mjs` — the exact phrasing the contracting
// authority's form uses.
const REQUIRED_KEYS = [
  'espd.readiness.title',
  'espd.readiness.progress',
  'espd.readiness.ready',
  'espd.readiness.companyTodo',
  'espd.readiness.companyTodoHint',
  'espd.readiness.bidTodo',
  'espd.readiness.noneLeft',
  'espd.readiness.reviewDeclarations',
  'espd.readiness.confirmDeclarations',
  'espd.readiness.export',
  'espd.readiness.exportHint',
  'espd.gap.openDossier',
  'espd.gap.openBid',
  'espd.declarations.title',
  'espd.declarations.incomplete',
  'espd.declarations.openDossier',
  'espd.declarations.declaredOn',
  'espd.declarations.neverConfirmed',
  'espd.declarations.stale',
  'espd.declarations.appliesTitle',
  'espd.declarations.selfCleaning',
  'espd.declarations.allTitle',
  'espd.declarations.applies',
  'espd.declarations.doesNotApply',
  'espd.declarations.unanswered',
  'espd.declarations.confirm',
  'espd.declarations.confirming',
  'espd.declarations.changed',
  'espd.declarations.error',
  'espd.export.title',
  'espd.export.notReadyHint',
  'espd.export.exporting',
  'espd.export.nextStep',
  'espd.export.nextStepNoDeadline',
  'espd.export.history',
  'espd.export.historyHint',
  'espd.export.notEntitled',
  'espd.export.notReady',
  'espd.export.error',
  'espd.dossier.title',
  'espd.dossier.progress',
  'espd.dossier.intro',
  'espd.dossier.applies',
  'espd.dossier.doesNotApply',
  'espd.dossier.unanswered',
  'espd.dossier.selfCleaning',
  'espd.dossier.selfCleaningHint',
  'espd.dossier.statedAt',
  'espd.dossier.error',
  'espd.fields.is_sme',
  'espd.fields.representative',
  'espd.fields.buyer_name',
  'espd.fields.reference',
  'espd.fields.lot_ref',
  'espd.fields.confirmation',
  'espd.fields.criterion',
] as const;

describe('espd locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every required espd key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });

  it.each(entries)('%s names every criterion the document can carry', (_path, mod) => {
    for (const criterion of ESPD_CRITERIA) {
      const label = get(mod.default, `espd.criteria.${criterion}`);
      expect(label, criterion).toBeTruthy();
      // A label that is just the key back would satisfy `toBeTruthy` and read as
      // a bug on a signed document.
      expect(label, criterion).not.toBe(criterion);
    }
  });

  it.each(entries)('%s explains every gap reason', (_path, mod) => {
    for (const reason of GAP_REASONS) {
      expect(get(mod.default, `espd.gap.reason.${reason}`), reason).toBeTruthy();
    }
  });

  it.each(entries)('%s heads every Part III section', (_path, mod) => {
    for (const part of DECLARATION_PARTS) {
      expect(get(mod.default, `espd.parts.${part}`), part).toBeTruthy();
    }
  });

  // The sheet offers three of these four; the export history renders whatever
  // the server recorded, and the server will render a PDF against either data
  // model — so all four must have a label.
  it.each(entries)('%s labels every export version and format', (_path, mod) => {
    for (const version of ESPD_VERSIONS) {
      for (const format of ESPD_FORMATS) {
        const key = `espd.export.choice.${version}_${format}`;
        expect(get(mod.default, key), key).toBeTruthy();
      }
    }
    for (const format of ESPD_FORMATS) {
      expect(get(mod.default, `espd.export.hint.${format}`), format).toBeTruthy();
    }
  });

  it.each(entries)('%s keeps the interpolations the UI passes', (_path, mod) => {
    for (const token of ['{{ready}}', '{{total}}']) {
      expect(get(mod.default, 'espd.readiness.progress'), token).toContain(token);
    }
    for (const token of ['{{answered}}', '{{total}}']) {
      expect(get(mod.default, 'espd.dossier.progress'), token).toContain(token);
    }
    for (const key of [
      'espd.declarations.declaredOn',
      'espd.dossier.statedAt',
      'espd.export.nextStep',
    ]) {
      expect(get(mod.default, key), key).toContain('{{when}}');
    }
  });
});

describe('espd vocabularies', () => {
  // The dossier form renders the grounds grouped, one section per part, and
  // filters by this function. A ground whose key does not resolve to one of the
  // three sections would be silently dropped from the form — and an exclusion
  // ground nobody is asked about is the failure that matters here.
  it('files every exclusion ground under a rendered Part III heading', () => {
    for (const criterion of EXCLUSION_CRITERIA) {
      expect(DECLARATION_PARTS, criterion).toContain(partOfCriterion(criterion));
    }
  });
});
