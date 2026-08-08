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

// The scheda gara renders every one of these domain enums by code — the wire
// carries stable strings (`company.proto` keeps them as strings, not proto
// enums, so a constant rename is not a wire break), and the UI owns all 24
// renderings. Mirroring the Go constant sets here is the only mechanical link
// between a domain enum and its translations: add a `company.GapStatus` or a
// `company.RemedyKind` in Go without a translation and this suite fails in all
// 24 locales at once, instead of the UI quietly printing the raw snake_case
// code — which, for an eligibility verdict, is a lie about eligibility.
const GAP_STATUSES = ['met', 'missing', 'shortfall', 'expiring', 'expired', 'unknown'] as const;
const REMEDY_KINDS = [
  'provide_fact',
  'renew',
  'upgrade',
  'avvalimento',
  'rti',
  'subcontract',
] as const;
const CAVEATS = [
  'no_requirements_captured',
  'award_grid_only',
  'unconfirmed_extraction',
  'open_questions',
  'documents_unread',
] as const;
const VERDICTS = ['insufficient_data', 'go', 'no_go'] as const;
// 1:1 with `Requirement.Kind`, and the capture question is routed off it — a
// kind with no prompt is a gap the user can never answer.
const REQUIREMENT_KINDS = [
  'soa',
  'certification',
  'turnover',
  'past_contracts',
  'headcount',
  'registration',
  'other',
] as const;
const REQUIREMENT_SOURCES = [
  'user_stated',
  'document_excerpt',
  'award_criteria',
  'agent_inferred',
] as const;
const PROVENANCES = ['user_stated', 'user_confirmed', 'agent_inferred', 'imported'] as const;

const REQUIRED_KEYS = [
  'company.dossier.title',
  'company.dossier.description',
  'company.dossier.save',
  'company.dossier.saving',
  'company.dossier.error',
  'company.dossier.identity.legalName',
  'company.dossier.identity.vatNumber',
  'company.dossier.identity.fiscalCode',
  'company.dossier.identity.legalForm',
  'company.dossier.identity.cciaaOffice',
  'company.dossier.identity.cciaaNumber',
  'company.dossier.identity.country',
  'company.dossier.identity.nuts',
  'company.dossier.identity.foundedYear',
  'company.dossier.soa.title',
  'company.dossier.soa.category',
  'company.dossier.soa.classifica',
  'company.dossier.soa.issuedBy',
  'company.dossier.soa.validUntil',
  'company.dossier.soa.verifiedUntil',
  'company.dossier.soa.verifiedUntilHint',
  'company.dossier.soa.add',
  'company.dossier.soa.remove',
  'company.dossier.soa.empty',
  'company.dossier.certifications.title',
  'company.dossier.certifications.standard',
  'company.dossier.certifications.standardRaw',
  'company.dossier.certifications.scope',
  'company.dossier.certifications.issuedBy',
  'company.dossier.certifications.validFrom',
  'company.dossier.certifications.validUntil',
  'company.dossier.certifications.add',
  'company.dossier.certifications.remove',
  'company.dossier.certifications.empty',
  'company.dossier.financialYears.title',
  'company.dossier.financialYears.year',
  'company.dossier.financialYears.turnover',
  'company.dossier.financialYears.specificTurnover',
  'company.dossier.financialYears.currency',
  'company.dossier.financialYears.headcount',
  'company.dossier.financialYears.add',
  'company.dossier.financialYears.remove',
  'company.dossier.financialYears.empty',
  'company.dossier.pastContracts.title',
  'company.dossier.pastContracts.description',
  'company.dossier.pastContracts.buyer',
  'company.dossier.pastContracts.cpv',
  'company.dossier.pastContracts.value',
  'company.dossier.pastContracts.started',
  'company.dossier.pastContracts.ended',
  'company.dossier.pastContracts.role',
  'company.dossier.pastContracts.sharePct',
  'company.dossier.pastContracts.sharePctHint',
  'company.dossier.pastContracts.add',
  'company.dossier.pastContracts.remove',
  'company.dossier.pastContracts.empty',
  'company.dossier.registrations.title',
  'company.dossier.registrations.kind',
  'company.dossier.registrations.authority',
  'company.dossier.registrations.identifier',
  'company.dossier.registrations.section',
  'company.dossier.registrations.validFrom',
  'company.dossier.registrations.validUntil',
  'company.dossier.registrations.add',
  'company.dossier.registrations.remove',
  'company.dossier.registrations.empty',
  'company.provenance.never',
  'company.eligibility.title',
  'company.eligibility.recheck',
  'company.eligibility.evaluatedAt',
  'company.eligibility.requirementLabel',
  'company.eligibility.heldLabel',
  'company.eligibility.changesLabel',
  'company.eligibility.caveatsLabel',
  'company.eligibility.lead.requires',
  'company.eligibility.lead.holds',
  'company.eligibility.lead.noRequirements',
  'company.requirements.title',
  'company.requirements.empty',
  'company.requirements.confirm',
  'company.requirements.confirmed',
  'company.requirements.remove',
  'company.requirements.unconfirmedHint',
  'company.requirements.lotLabel',
  'company.requirements.noticeLevel',
  'company.requirements.askWhy',
  'company.capture.title',
  'company.capture.save',
  'company.capture.saving',
  'company.capture.notSure',
  'company.capture.error',
  'company.capture.cannotCheck',
] as const;

// Every locale must define at least `_one` and `_other`; CLDR languages that
// need extra categories (few/many/two/zero) carry them too, but only the two
// universal suffixes are demanded here.
const PLURAL_STEMS = [
  'company.eligibility.counts.requirements',
  'company.eligibility.counts.authoritative',
  'company.eligibility.counts.blocking',
  'company.eligibility.counts.unknown',
] as const;

describe('company locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every required company key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });

  it.each(entries)('%s translates every gap status and remedy kind', (_path, mod) => {
    for (const code of GAP_STATUSES) {
      expect(get(mod.default, `company.eligibility.status.${code}`), code).toBeTruthy();
    }
    for (const code of REMEDY_KINDS) {
      expect(get(mod.default, `company.eligibility.remedy.${code}`), code).toBeTruthy();
    }
  });

  it.each(entries)('%s translates every verdict and caveat', (_path, mod) => {
    for (const code of VERDICTS) {
      expect(get(mod.default, `company.eligibility.verdict.${code}`), code).toBeTruthy();
    }
    for (const code of CAVEATS) {
      expect(get(mod.default, `company.eligibility.caveat.${code}`), code).toBeTruthy();
    }
  });

  it.each(entries)('%s prompts for every requirement kind', (_path, mod) => {
    for (const code of REQUIREMENT_KINDS) {
      expect(get(mod.default, `company.capture.prompt.${code}`), code).toBeTruthy();
    }
  });

  it.each(entries)('%s labels every requirement source and provenance', (_path, mod) => {
    for (const code of REQUIREMENT_SOURCES) {
      expect(get(mod.default, `company.requirements.source.${code}`), code).toBeTruthy();
    }
    for (const code of PROVENANCES) {
      expect(get(mod.default, `company.provenance.${code}`), code).toBeTruthy();
    }
  });

  it.each(entries)('%s keeps the interpolations the UI passes', (_path, mod) => {
    expect(get(mod.default, 'company.eligibility.evaluatedAt'), 'evaluatedAt').toContain(
      '{{when}}',
    );
    expect(get(mod.default, 'company.requirements.lotLabel'), 'lotLabel').toContain('{{lot}}');
  });

  it.each(entries)('%s defines the required plural forms', (_path, mod) => {
    for (const stem of PLURAL_STEMS) {
      for (const suffix of ['one', 'other'] as const) {
        const form = get(mod.default, `${stem}_${suffix}`);
        expect(form, `${stem}_${suffix}`).toBeTruthy();
        expect(form, `${stem}_${suffix}`).toContain('{{count}}');
      }
      // Extra CLDR categories are optional per language; when present they must
      // interpolate the count too.
      for (const suffix of ['two', 'few', 'many', 'zero'] as const) {
        const form = get(mod.default, `${stem}_${suffix}`);
        if (form !== undefined) {
          expect(form, `${stem}_${suffix}`).toContain('{{count}}');
        }
      }
    }
  });
});
