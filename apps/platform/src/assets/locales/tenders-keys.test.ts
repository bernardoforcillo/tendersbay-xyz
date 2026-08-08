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

// Non-plural keys every locale must define.
const REQUIRED_KEYS = [
  'tenders.searching',
  'tenders.empty.title',
  'tenders.empty.description',
  'tenders.error',
  'tenders.loadMore',
  'tenders.deadline.today',
  'tenders.deadline.expired',
  'tenders.value.unknown',
  'tenders.status.open',
  'tenders.status.awarded',
  'tenders.status.cancelled',
  'tenders.status.closed',
  'tenders.status.unknown',
  'tenders.detail.overview',
  'tenders.detail.buyer',
  'tenders.detail.value',
  'tenders.detail.published',
  'tenders.detail.deadline',
  'tenders.detail.status',
  'tenders.detail.procedure',
  'tenders.detail.cpv',
  'tenders.detail.cpvSecondary',
  'tenders.detail.region',
  'tenders.detail.language',
  'tenders.detail.documents',
  'tenders.detail.officialNotice',
  'tenders.detail.viewSource',
  'tenders.detail.lots',
  'tenders.detail.lotsNone',
  'tenders.detail.related',
  'tenders.detail.relatedNone',
  'tenders.detail.notFound',
  'tenders.detail.backToSearch',
  'tenders.detail.searchPlaceholder',
  'tenders.detail.meta.title',
  'tenders.detail.meta.description',
  'tenders.filters.country',
  'tenders.filters.anyCountry',
  'tenders.filters.sector',
  'tenders.filters.anySector',
  'tenders.filters.sectors.construction',
  'tenders.filters.sectors.it',
  'tenders.filters.sectors.health',
  'tenders.filters.sectors.energy',
  'tenders.filters.sectors.transport',
  'tenders.filters.sectors.education',
  'tenders.filters.sectors.environment',
  'tenders.filters.sectors.business',
  'tenders.filters.status',
  'tenders.filters.anyStatus',
  'tenders.filters.deadline',
  'tenders.filters.anyDeadline',
  'tenders.filters.deadline7',
  'tenders.filters.deadline30',
  'tenders.filters.deadline90',
  'tenders.filters.clear',
  'tenders.filters.sort',
  'tenders.filters.sorts.relevance',
  'tenders.filters.sorts.deadline',
  'tenders.filters.sorts.published',
  'tenders.filters.sorts.value',
  'tenders.filters.valueMin',
  'tenders.filters.valueMax',
  'tenders.filters.valuePlaceholder',
  'tenders.filters.buyer',
  'tenders.filters.buyerPlaceholder',
  'tenders.degraded',
  'tenders.applied.label',
  'tenders.applied.cpv',
  'tenders.applied.valueRange',
  'tenders.applied.valueMax',
  'tenders.applied.valueMin',
  'tenders.applied.deadlineUntil',
  'tenders.applied.undo',
  'tenders.applied.remove',
  'tenders.fit.tier.strong',
  'tenders.fit.tier.possible',
  'tenders.fit.tier.longShot',
  'tenders.fit.reasonSector',
  'tenders.fit.reasonCountry',
  'tenders.fit.reasonValueInBand',
  'tenders.fit.reasonValueBelow',
  'tenders.fit.reasonValueAbove',
  'tenders.fit.reasonRegion',
  'tenders.fit.reasonProcedure',
  'tenders.coverage.title',
  'tenders.coverage.openBuyerDocuments',
  'tenders.criteria.title',
  'tenders.criteria.notEnriched',
  'tenders.criteria.gridUnusable',
  'tenders.criteria.weight',
  'tenders.criteria.noWeight',
  'tenders.criteria.lot',
  'tenders.criteria.appliesToEveryLot',
  'tenders.criteria.showAllLots',
  'tenders.citation.page',
  'tenders.citation.pageRange',
  'tenders.citation.section',
  'tenders.citation.part',
  'tenders.citation.openDocument',
  'tenders.citation.expandPassage',
  'tenders.citation.collapsePassage',
  'tenders.citation.passageUnavailable',
  'tenders.citation.truncated',
] as const;

// `document.Coverage` and `document.Reason` are orthogonal on the wire: the
// badge renders the quantity on one line and the cause on the other, and a
// missing cause string would render the raw snake_case code — for a coverage
// cause that is a lie about what we read. Mirroring the Go constant sets here
// makes adding one without a translation fail the suite in all 24 locales.
const COVERAGE_VALUES = ['full', 'notice_only', 'none'] as const;
const COVERAGE_REASONS = [
  'no_documents_published',
  'notice_not_read',
  'body_not_retrieved',
  'not_yet_extracted',
  'extraction_failed',
] as const;
const CRITERION_TYPES = ['quality', 'cost', 'price'] as const;

// Plural key stems: every locale must define at least `_one` and `_other`;
// CLDR languages that need extra categories (few/many/two/zero) carry them
// too, but the completeness test only demands the two universal suffixes.
const PLURAL_STEMS = [
  'tenders.results',
  'tenders.deadline.days',
  'tenders.coverage.counts.documents',
  'tenders.coverage.counts.parts',
  'tenders.coverage.counts.knownLinks',
] as const;

describe('tenders locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every tenders key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });

  it.each(entries)('%s translates every coverage value and cause', (_path, mod) => {
    for (const code of COVERAGE_VALUES) {
      expect(get(mod.default, `tenders.coverage.label.${code}`), code).toBeTruthy();
    }
    for (const code of COVERAGE_REASONS) {
      expect(get(mod.default, `tenders.coverage.reason.${code}`), code).toBeTruthy();
    }
  });

  it.each(entries)('%s translates every award criterion type', (_path, mod) => {
    for (const code of CRITERION_TYPES) {
      expect(get(mod.default, `tenders.criteria.type.${code}`), code).toBeTruthy();
    }
  });

  it.each(entries)('%s keeps the citation interpolations', (_path, mod) => {
    expect(get(mod.default, 'tenders.citation.page'), 'page').toContain('{{page}}');
    expect(get(mod.default, 'tenders.citation.pageRange'), 'pageRange').toContain('{{start}}');
    expect(get(mod.default, 'tenders.citation.pageRange'), 'pageRange').toContain('{{end}}');
    expect(get(mod.default, 'tenders.citation.section'), 'section').toContain('{{section}}');
    expect(get(mod.default, 'tenders.citation.part'), 'part').toContain('{{index}}');
    expect(get(mod.default, 'tenders.criteria.lot'), 'criteria.lot').toContain('{{lot}}');
  });

  it.each(entries)('%s defines the required plural forms', (_path, mod) => {
    for (const stem of PLURAL_STEMS) {
      // Every locale in this app has CLDR `one` and `other` categories.
      for (const suffix of ['one', 'other'] as const) {
        const form = get(mod.default, `${stem}_${suffix}`);
        expect(form, `${stem}_${suffix}`).toBeTruthy();
        expect(form, `${stem}_${suffix}`).toContain('{{count}}');
      }
      // Extra CLDR categories are optional per language, but when present
      // they must interpolate the count too.
      for (const suffix of ['two', 'few', 'many', 'zero'] as const) {
        const form = get(mod.default, `${stem}_${suffix}`);
        if (form !== undefined) {
          expect(form, `${stem}_${suffix}`).toContain('{{count}}');
        }
      }
    }
  });
});
