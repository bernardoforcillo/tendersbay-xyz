/**
 * Curated top-level CPV divisions offered as the explore "sector" filter. Each entry
 * maps a stable i18n key (labels live under `tenders.filters.sectors.<key>`) to the
 * two-digit CPV division prefix the backend matches — `TenderFilters.cpv` is a prefix
 * match, so "45" catches all of division 45 (construction).
 */
export const CPV_SECTORS = [
  { key: 'construction', prefix: '45' },
  { key: 'it', prefix: '72' },
  { key: 'health', prefix: '85' },
  { key: 'energy', prefix: '09' },
  { key: 'transport', prefix: '60' },
  { key: 'education', prefix: '80' },
  { key: 'environment', prefix: '90' },
  { key: 'business', prefix: '79' },
] as const;

export type CpvSectorKey = (typeof CPV_SECTORS)[number]['key'];

/** The CPV division prefix for a sector key, or undefined for an unknown/empty key. */
export function cpvPrefix(key: string): string | undefined {
  return CPV_SECTORS.find((sector) => sector.key === key)?.prefix;
}
