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
  'explore.clientProfile.sectors',
  'explore.clientProfile.countries',
  'explore.clientProfile.valueMin',
  'explore.clientProfile.valueMax',
  'explore.clientProfile.notes',
  'explore.clientProfile.notesPlaceholder',
  'explore.clientProfile.save',
  'explore.clientProfile.invalidBand',
  'explore.clientProfile.error',
  'explore.clientProfile.procedureTypesLabel',
  'explore.clientProfile.procedureTypes.open',
  'explore.clientProfile.procedureTypes.restricted',
  'explore.clientProfile.procedureTypes.negotiated',
  'explore.clientProfile.procedureTypes.competitive_dialogue',
  'explore.clientProfile.procedureTypes.innovation_partnership',
  'explore.clientProfile.procedureTypes.other',
  'explore.clientProfile.regions',
  'explore.clientProfile.regionsPlaceholder',
  'explore.clientProfile.regionsHint',
  'explore.shortlist.title',
  'explore.shortlist.emptyTitle',
  'explore.shortlist.emptyDescription',
] as const;

describe('explore locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every explore key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });
});
