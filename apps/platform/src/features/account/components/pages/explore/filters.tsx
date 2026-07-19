import { Button, Select } from '@tendersbay/components/core';
import { useTranslation } from 'react-i18next';
import {
  CPV_SECTORS,
  countryName,
  cpvPrefix,
  TENDER_COUNTRY_CODES,
  type TenderFilterValues,
} from '~/features/account/components/organisms/tender-feed';
import { DEADLINE_PRESETS, type DeadlinePreset, deadlineRange } from './deadline-preset';

/** Status values offered as filters (excludes the display-only "unknown"). */
const STATUSES = ['open', 'awarded', 'cancelled', 'closed'] as const;

export type ExploreFilterKey = 'country' | 'sector' | 'status' | 'deadline';

/** The filter bar's raw UI selections; '' means "any" for each control. */
export type FilterSelections = {
  country: string;
  sector: string;
  status: string;
  deadline: string;
};

export const EMPTY_FILTERS: FilterSelections = {
  country: '',
  sector: '',
  status: '',
  deadline: '',
};

export function hasActiveFilters(s: FilterSelections): boolean {
  return Boolean(s.country || s.sector || s.status || s.deadline);
}

/**
 * Maps raw UI selections to the request `TenderFilterValues`, omitting unset fields
 * (an empty field means "no constraint", so it must not be sent). The deadline preset
 * is resolved against `now` into an RFC3339 from/to window.
 */
export function toFilterValues(s: FilterSelections, now: Date): TenderFilterValues {
  const values: TenderFilterValues = {};
  if (s.country) values.country = s.country;
  if (s.sector) {
    const prefix = cpvPrefix(s.sector);
    if (prefix) values.cpv = prefix;
  }
  if (s.status) values.status = s.status;
  if (s.deadline) {
    const range = deadlineRange(Number(s.deadline) as DeadlinePreset, now);
    if (range) {
      values.deadlineFrom = range.from;
      values.deadlineTo = range.to;
    }
  }
  return values;
}

/**
 * The explore search filter bar: Country, Sector (CPV), Status and Deadline, each a
 * native `Select` defaulting to "any". A change bubbles up via `onChange` (the page
 * owns the state and re-runs the search); "Clear all" appears only when a filter is set.
 */
export function ExploreFilters({
  value,
  locale,
  onChange,
  onClear,
}: {
  value: FilterSelections;
  locale: string;
  onChange: (key: ExploreFilterKey, next: string) => void;
  onClear: () => void;
}) {
  const { t } = useTranslation();

  const countries = TENDER_COUNTRY_CODES.map((code) => ({
    code,
    name: countryName(code, locale),
  })).sort((a, b) => a.name.localeCompare(b.name, locale));

  return (
    <div className="flex flex-wrap items-end justify-center gap-3">
      <Select
        label={t('tenders.filters.country')}
        value={value.country}
        onChange={(e) => onChange('country', e.target.value)}
      >
        <option value="">{t('tenders.filters.anyCountry')}</option>
        {countries.map(({ code, name }) => (
          <option key={code} value={code}>
            {name}
          </option>
        ))}
      </Select>

      <Select
        label={t('tenders.filters.sector')}
        value={value.sector}
        onChange={(e) => onChange('sector', e.target.value)}
      >
        <option value="">{t('tenders.filters.anySector')}</option>
        {CPV_SECTORS.map(({ key }) => (
          <option key={key} value={key}>
            {t(`tenders.filters.sectors.${key}`)}
          </option>
        ))}
      </Select>

      <Select
        label={t('tenders.filters.status')}
        value={value.status}
        onChange={(e) => onChange('status', e.target.value)}
      >
        <option value="">{t('tenders.filters.anyStatus')}</option>
        {STATUSES.map((status) => (
          <option key={status} value={status}>
            {t(`tenders.status.${status}`)}
          </option>
        ))}
      </Select>

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

      {hasActiveFilters(value) && (
        <Button variant="ghost" onPress={onClear}>
          {t('tenders.filters.clear')}
        </Button>
      )}
    </div>
  );
}
