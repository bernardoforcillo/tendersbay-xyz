import { describe, expect, it } from 'vitest';

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

const REQUIRED_KEYS = [
  'bid.stage.shortlisted',
  'bid.stage.preparing',
  'bid.stage.submitted',
  'bid.goNoGo.undecided',
  'bid.goNoGo.go',
  'bid.goNoGo.noGo',
  'bid.outcome.won',
  'bid.outcome.lost',
  'bid.outcome.withdrawn',
  'bid.bands.needsYouNow',
  'bid.bands.active',
  'bid.bands.submitted',
  'bid.bands.closed',
  'bid.actions.prepare',
  'bid.empty.title',
  'bid.empty.needsProfileTitle',
  'bid.empty.needsProfileCta',
  'bid.empty.seedTitle',
  'bid.tombstone.title',
  'bid.tombstone.description',
  'bid.checklist.title',
  'bid.checklist.statusLabel',
  'bid.checklist.noteLabel',
  'bid.checklist.required',
  'bid.checklist.status.pending',
  'bid.checklist.status.done',
  'bid.checklist.status.na',
  'bid.detail.goNoGoTitle',
  'bid.detail.stageTitle',
  'bid.detail.outcomeTitle',
  'bid.detail.backToPortfolio',
  'bid.picker.title',
  'bid.picker.empty',
  'bid.fit.needsProfile',
  'bid.errors.duplicate',
  'bid.errors.generic',
  'bid.scheda.coverageTitle',
  'bid.scheda.evidenceTitle',
  'bid.scheda.pointsTitle',
  'bid.scheda.repairChat.title',
  'bid.scheda.repairChat.open',
  'bid.scheda.repairChat.close',
  'bid.scheda.repairChat.askWhy',
  'bid.scheda.repairChat.seedGapQuestion',
  'bid.decision.recordedAgainst',
  'bid.decision.youDecided',
  'bid.decision.noRecommendation',
  'bid.decision.recordedAt',
] as const;

const CHECKLIST_SECTIONS = ['part_ii', 'part_iii', 'part_iv', 'conclusion'] as const;
const CHECKLIST_ITEMS = [
  'identification',
  'sme_status',
  'representation',
  'criminal_convictions',
  'tax_payments',
  'social_security',
  'insolvency',
  'misconduct',
  'conflict_interest',
  'suitability',
  'economic_standing',
  'technical_ability',
  'quality_assurance',
  'espd_signed',
] as const;

describe('bid locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every required bid key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });

  it.each(entries)('%s labels every checklist section and item code', (_path, mod) => {
    for (const code of CHECKLIST_SECTIONS) {
      expect(get(mod.default, `bid.checklist.sections.${code}`), code).toBeTruthy();
    }
    for (const code of CHECKLIST_ITEMS) {
      expect(get(mod.default, `bid.checklist.items.${code}`), code).toBeTruthy();
    }
  });

  it.each(
    entries,
  )('%s keeps the {{done}}/{{total}} placeholders in checklist.progress', (_path, mod) => {
    const progress = get(mod.default, 'bid.checklist.progress');
    expect(progress, 'checklist.progress').toContain('{{done}}');
    expect(progress, 'checklist.progress').toContain('{{total}}');
  });

  // The recorded go/no-go baseline is three separate facts — the recommendation
  // it was recorded against, the decision the user actually made, and when.
  // Dropping an interpolation collapses them into an unreadable line.
  it.each(entries)('%s keeps the decision interpolations', (_path, mod) => {
    expect(get(mod.default, 'bid.decision.recordedAgainst'), 'recordedAgainst').toContain(
      '{{recommendation}}',
    );
    expect(get(mod.default, 'bid.decision.youDecided'), 'youDecided').toContain('{{decision}}');
    expect(get(mod.default, 'bid.decision.recordedAt'), 'recordedAt').toContain('{{when}}');
    expect(get(mod.default, 'bid.scheda.repairChat.seedGapQuestion'), 'seedGapQuestion').toContain(
      '{{requirement}}',
    );
  });

  it.each(entries)('%s defines the blocking-gap plural forms', (_path, mod) => {
    const stem = 'bid.decision.blockingGapsAtDecision';
    for (const suffix of ['one', 'other'] as const) {
      const form = get(mod.default, `${stem}_${suffix}`);
      expect(form, `${stem}_${suffix}`).toBeTruthy();
      expect(form, `${stem}_${suffix}`).toContain('{{count}}');
    }
    for (const suffix of ['two', 'few', 'many', 'zero'] as const) {
      const form = get(mod.default, `${stem}_${suffix}`);
      if (form !== undefined) {
        expect(form, `${stem}_${suffix}`).toContain('{{count}}');
      }
    }
  });
});
