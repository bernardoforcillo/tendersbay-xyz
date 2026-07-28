import { cn } from '@tendersbay/components/core';
import type { TenderFilters } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';
import { countryName } from '~/features/account/components/organisms/tender-feed';

/**
 * Shows the constraints the server lifted out of the query text — typing
 * "pulizie sotto 100k" filters by budget, and this is where that becomes
 * visible.
 *
 * It renders ONLY inferred constraints, never the ones the user picked in the
 * filter bar: those are already on screen, and echoing them here would suggest
 * the search added something it didn't. A filter the user can't see is a filter
 * they can't correct, which is the whole reason this exists.
 */
export function AppliedFilterChips({
  applied,
  explicit,
  locale,
  onClear,
}: {
  /** What the search actually ran with, as reported by the server. */
  applied?: TenderFilters;
  /** What the user set in the filter bar, to be subtracted out. */
  explicit: {
    countries: string[];
    cpvPrefixes: string[];
    statuses: string[];
    hasValueBounds: boolean;
    hasDeadline: boolean;
  };
  locale: string;
  /** Clears the query text, which is where these constraints came from. */
  onClear: () => void;
}) {
  const { t } = useTranslation();
  if (!applied) return null;

  const chips: string[] = [];

  for (const code of applied.countries) {
    if (!explicit.countries.includes(code)) chips.push(countryName(code, locale));
  }
  for (const prefix of applied.cpvPrefixes) {
    if (!explicit.cpvPrefixes.includes(prefix)) {
      chips.push(t('tenders.applied.cpv', { code: prefix }));
    }
  }
  for (const status of applied.statuses) {
    if (!explicit.statuses.includes(status)) chips.push(t(`tenders.status.${status}`));
  }
  if (!explicit.hasValueBounds) {
    const min = applied.valueMin;
    const max = applied.valueMax;
    // Formatted without a currency: the bound is a plain number the parser
    // read out of the query, and stamping a currency on it would assert
    // something the query never said.
    const format = (v: bigint) => new Intl.NumberFormat(locale).format(v);
    if (min !== undefined && max !== undefined) {
      chips.push(t('tenders.applied.valueRange', { min: format(min), max: format(max) }));
    } else if (max !== undefined) {
      chips.push(t('tenders.applied.valueMax', { max: format(max) }));
    } else if (min !== undefined) {
      chips.push(t('tenders.applied.valueMin', { min: format(min) }));
    }
  }
  if (!explicit.hasDeadline && applied.deadlineTo) {
    const until = new Date(applied.deadlineTo);
    if (!Number.isNaN(until.getTime())) {
      chips.push(
        t('tenders.applied.deadlineUntil', {
          date: new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(until),
        }),
      );
    }
  }

  if (chips.length === 0) return null;

  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      <span className="text-ink-500">{t('tenders.applied.label')}</span>
      {chips.map((chip) => (
        <span
          key={chip}
          className={cn(
            'rounded-full border border-brand-200 bg-brand-50 px-2.5 py-0.5',
            'text-xs font-medium text-brand-800',
          )}
        >
          {chip}
        </span>
      ))}
      <button
        type="button"
        onClick={onClear}
        className="text-xs text-ink-500 underline underline-offset-2 outline-none hover:text-ink-700 focus-visible:ring-2 focus-visible:ring-brand-600"
      >
        {t('tenders.applied.undo')}
      </button>
    </div>
  );
}
