import { Button, cn, Select } from '@tendersbay/components/core';
import type { FacetCount } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';
import {
  CPV_SECTORS,
  countryName,
  cpvPrefix,
  TENDER_COUNTRY_CODES,
  type TenderFilterValues,
  type TenderSort,
} from '~/features/account/components/organisms/tender-feed';
import { DEADLINE_PRESETS, type DeadlinePreset, deadlineRange } from './deadline-preset';

/** Status values offered as filters (excludes the display-only "unknown"). */
const STATUSES = ['open', 'awarded', 'cancelled', 'closed'] as const;

/** Result orderings offered in the sort control. */
const SORTS = ['relevance', 'deadline', 'published', 'value'] as const;

export type ExploreFilterKey =
  | 'countries'
  | 'sectors'
  | 'statuses'
  | 'deadline'
  | 'valueMin'
  | 'valueMax'
  | 'buyer'
  | 'sort';

/**
 * The filter bar's raw UI selections.
 *
 * Country, sector and status are multi-select: a bidder works across several
 * countries and sectors at once, and forcing one at a time made them run the
 * same search repeatedly. An empty list means "any". Value bounds are kept as
 * the raw strings the inputs hold, so a half-typed number never becomes a `0`
 * that silently filters everything out.
 */
export type FilterSelections = {
  countries: string[];
  sectors: string[];
  statuses: string[];
  deadline: string;
  valueMin: string;
  valueMax: string;
  buyer: string;
  sort: TenderSort;
};

export const EMPTY_FILTERS: FilterSelections = {
  countries: [],
  sectors: [],
  statuses: [],
  deadline: '',
  valueMin: '',
  valueMax: '',
  buyer: '',
  sort: 'relevance',
};

/**
 * Whether anything narrows the search. Sort is excluded on purpose: reordering
 * results is not filtering them, so it must not light up "clear filters".
 */
export function hasActiveFilters(s: FilterSelections): boolean {
  return Boolean(
    s.countries.length ||
      s.sectors.length ||
      s.statuses.length ||
      s.deadline ||
      s.valueMin ||
      s.valueMax ||
      s.buyer.trim(),
  );
}

/**
 * Parses a money input into MINOR units, returning undefined for anything that
 * isn't a number.
 *
 * The input is whole euros — that is what the field asks for and what the user
 * types — while `value_min`/`value_max` are compared against a minor-unit
 * column server-side, so the bound is scaled here at the edge. Sending the
 * typed number unscaled capped a "100000" search at €1,000.
 */
function parseMoney(raw: string): bigint | undefined {
  const cleaned = raw.replace(/[^\d]/g, '');
  if (!cleaned) return undefined;
  return BigInt(cleaned) * 100n;
}

/**
 * Maps raw UI selections to the request `TenderFilterValues`, omitting unset
 * fields (an empty field means "no constraint", so it must not be sent). The
 * deadline preset is resolved against `now` into an RFC3339 from/to window.
 */
export function toFilterValues(s: FilterSelections, now: Date): TenderFilterValues {
  const values: TenderFilterValues = {};
  if (s.countries.length) values.countries = s.countries;
  if (s.sectors.length) {
    const prefixes = s.sectors.map(cpvPrefix).filter((p): p is string => Boolean(p));
    if (prefixes.length) values.cpvPrefixes = prefixes;
  }
  if (s.statuses.length) values.statuses = s.statuses;
  if (s.deadline) {
    const range = deadlineRange(Number(s.deadline) as DeadlinePreset, now);
    if (range) {
      values.deadlineFrom = range.from;
      values.deadlineTo = range.to;
    }
  }
  const min = parseMoney(s.valueMin);
  const max = parseMoney(s.valueMax);
  if (min !== undefined) values.valueMin = min;
  if (max !== undefined) values.valueMax = max;
  if (s.buyer.trim()) values.buyer = s.buyer.trim();
  return values;
}

/** Toggles one value in a multi-select list. */
export function toggleSelection(current: string[], value: string): string[] {
  return current.includes(value) ? current.filter((v) => v !== value) : [...current, value];
}

/** Looks up a facet count, or undefined when the server sent none. */
function facetCount(facets: FacetCount[] | undefined, value: string): number | undefined {
  return facets?.find((f) => f.value === value)?.count;
}

/**
 * A multi-select rendered as a row of toggle chips rather than a `<select multiple>`,
 * which is close to unusable on touch and hides the current selection behind a
 * scroll. Chips keep every active choice visible, which matters most here: an
 * invisible filter is one the user can’t tell is narrowing their results.
 */
function ChipGroup({
  label,
  options,
  selected,
  onToggle,
}: {
  label: string;
  options: { value: string; label: string; count?: number }[];
  selected: string[];
  onToggle: (value: string) => void;
}) {
  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="text-xs font-medium text-ink-500">{label}</legend>
      <div className="flex flex-wrap gap-1.5">
        {options.map(({ value, label: optionLabel, count }) => {
          const isSelected = selected.includes(value);
          return (
            <button
              key={value}
              type="button"
              aria-pressed={isSelected}
              onClick={() => onToggle(value)}
              className={cn(
                'rounded-full border px-3 py-1 text-sm transition-colors outline-none',
                'focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2',
                isSelected
                  ? 'border-brand-600 bg-brand-600 text-white'
                  : 'border-ink-200 bg-white/70 text-ink-700 hover:border-ink-300',
              )}
            >
              {optionLabel}
              {count !== undefined && (
                <span
                  className={cn(
                    'ml-1.5 tabular-nums',
                    isSelected ? 'text-white/70' : 'text-ink-400',
                  )}
                >
                  {count}
                </span>
              )}
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}

/**
 * The explore search filter bar.
 *
 * `facets` are per-value result counts from the last search. They are advisory:
 * the server computes them over the window it actually ranked for a query-driven
 * search, so a missing count means "not in this window", never "zero results" —
 * which is why an option with no count still renders and stays selectable.
 */
export function ExploreFilters({
  value,
  locale,
  facets,
  onChange,
  onClear,
}: {
  value: FilterSelections;
  locale: string;
  facets?: {
    countries: FacetCount[];
    statuses: FacetCount[];
    cpvDivisions: FacetCount[];
  };
  onChange: (key: ExploreFilterKey, next: string[] | string) => void;
  onClear: () => void;
}) {
  const { t } = useTranslation();

  const countries = TENDER_COUNTRY_CODES.map((code) => ({
    value: code,
    label: countryName(code, locale),
    count: facetCount(facets?.countries, code),
  })).sort((a, b) => a.label.localeCompare(b.label, locale));

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
      <div className="flex flex-wrap items-start gap-x-6 gap-y-4">
        <ChipGroup
          label={t('tenders.filters.country')}
          options={countries}
          selected={value.countries}
          onToggle={(code) => onChange('countries', toggleSelection(value.countries, code))}
        />
      </div>

      <div className="flex flex-wrap items-start gap-x-6 gap-y-4">
        <ChipGroup
          label={t('tenders.filters.sector')}
          options={CPV_SECTORS.map(({ key, prefix }) => ({
            value: key,
            label: t(`tenders.filters.sectors.${key}`),
            count: facetCount(facets?.cpvDivisions, prefix),
          }))}
          selected={value.sectors}
          onToggle={(key) => onChange('sectors', toggleSelection(value.sectors, key))}
        />

        <ChipGroup
          label={t('tenders.filters.status')}
          options={STATUSES.map((status) => ({
            value: status,
            label: t(`tenders.status.${status}`),
            count: facetCount(facets?.statuses, status),
          }))}
          selected={value.statuses}
          onToggle={(status) => onChange('statuses', toggleSelection(value.statuses, status))}
        />
      </div>

      <div className="flex flex-wrap items-end gap-3">
        <Select
          label={t('tenders.filters.deadline')}
          value={value.deadline}
          onChange={(e) => onChange('deadline', e.target.value)}
        >
          <option value="">{t('tenders.filters.anyDeadline')}</option>
          {DEADLINE_PRESETS.map((days) => (
            <option key={days} value={String(days)}>
              {t(`tenders.filters.deadline${days}`)}
            </option>
          ))}
        </Select>

        <Select
          label={t('tenders.filters.sort')}
          value={value.sort}
          onChange={(e) => onChange('sort', e.target.value)}
        >
          {SORTS.map((sort) => (
            <option key={sort} value={sort}>
              {t(`tenders.filters.sorts.${sort}`)}
            </option>
          ))}
        </Select>

        <label className="flex flex-col gap-1 text-xs font-medium text-ink-500">
          {t('tenders.filters.valueMin')}
          <input
            type="text"
            inputMode="numeric"
            value={value.valueMin}
            placeholder={t('tenders.filters.valuePlaceholder')}
            onChange={(e) => onChange('valueMin', e.target.value)}
            className="w-32 rounded-lg border border-ink-200 bg-white/70 px-3 py-2 text-sm text-ink-900 outline-none focus-visible:ring-2 focus-visible:ring-brand-600"
          />
        </label>

        <label className="flex flex-col gap-1 text-xs font-medium text-ink-500">
          {t('tenders.filters.valueMax')}
          <input
            type="text"
            inputMode="numeric"
            value={value.valueMax}
            placeholder={t('tenders.filters.valuePlaceholder')}
            onChange={(e) => onChange('valueMax', e.target.value)}
            className="w-32 rounded-lg border border-ink-200 bg-white/70 px-3 py-2 text-sm text-ink-900 outline-none focus-visible:ring-2 focus-visible:ring-brand-600"
          />
        </label>

        <label className="flex flex-col gap-1 text-xs font-medium text-ink-500">
          {t('tenders.filters.buyer')}
          <input
            type="text"
            value={value.buyer}
            placeholder={t('tenders.filters.buyerPlaceholder')}
            onChange={(e) => onChange('buyer', e.target.value)}
            className="w-56 rounded-lg border border-ink-200 bg-white/70 px-3 py-2 text-sm text-ink-900 outline-none focus-visible:ring-2 focus-visible:ring-brand-600"
          />
        </label>

        {hasActiveFilters(value) && (
          <Button variant="ghost" onPress={onClear}>
            {t('tenders.filters.clear')}
          </Button>
        )}
      </div>
    </div>
  );
}
