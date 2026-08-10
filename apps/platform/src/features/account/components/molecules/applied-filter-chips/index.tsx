import { cn } from '@tendersbay/components/core';
import type { TenderFilters } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';
import { countryName } from '~/features/account/components/organisms/tender-feed';

/** What the user set in the filter bar, to be subtracted out of `applied`. */
export type ExplicitFilterFacets = {
  countries: string[];
  cpvPrefixes: string[];
  statuses: string[];
  hasValueBounds: boolean;
  hasDeadline: boolean;
};

type AppliedFilterEntry =
  | { kind: 'country'; code: string }
  | { kind: 'cpv'; prefix: string }
  | { kind: 'status'; status: string }
  | { kind: 'valueRange'; min: bigint; max: bigint }
  | { kind: 'valueMax'; max: bigint }
  | { kind: 'valueMin'; min: bigint }
  | { kind: 'deadlineUntil'; date: Date };

/**
 * Computes which inferred constraints AppliedFilterChips would show — a
 * country/cpv/status/value/deadline the server applied that the user did NOT
 * already pick explicitly. Pulled out as its own pure function, rather than
 * inlined in the component, so it is the SINGLE source of truth for "does
 * this family have anything to show": the component uses it to render, and
 * the tenders page uses it to decide whether to render the shared
 * "Read from your search:" heading above both chip families, without
 * duplicating (and risking drifting from) these same conditions.
 */
export function appliedFilterEntries(
  applied: TenderFilters | undefined,
  explicit: ExplicitFilterFacets,
): AppliedFilterEntry[] {
  if (!applied) return [];

  const entries: AppliedFilterEntry[] = [];
  for (const code of applied.countries) {
    if (!explicit.countries.includes(code)) entries.push({ kind: 'country', code });
  }
  for (const prefix of applied.cpvPrefixes) {
    if (!explicit.cpvPrefixes.includes(prefix)) entries.push({ kind: 'cpv', prefix });
  }
  for (const status of applied.statuses) {
    if (!explicit.statuses.includes(status)) entries.push({ kind: 'status', status });
  }
  if (!explicit.hasValueBounds) {
    const min = applied.valueMin;
    const max = applied.valueMax;
    if (min !== undefined && max !== undefined) {
      entries.push({ kind: 'valueRange', min, max });
    } else if (max !== undefined) {
      entries.push({ kind: 'valueMax', max });
    } else if (min !== undefined) {
      entries.push({ kind: 'valueMin', min });
    }
  }
  if (!explicit.hasDeadline && applied.deadlineTo) {
    const until = new Date(applied.deadlineTo);
    // An unparseable date must not become a chip reading "closing by Invalid
    // Date" — silently dropping it is safer than showing garbage.
    if (!Number.isNaN(until.getTime())) entries.push({ kind: 'deadlineUntil', date: until });
  }
  return entries;
}

/**
 * Shows the constraints the server lifted out of the query text — typing
 * "pulizie sotto 100k" filters by budget, and this is where that becomes
 * visible.
 *
 * It renders ONLY inferred constraints, never the ones the user picked in the
 * filter bar: those are already on screen, and echoing them here would suggest
 * the search added something it didn't. A filter the user can't see is a filter
 * they can't correct, which is the whole reason this exists.
 *
 * Deliberately renders with NO leading "Read from your query:" label of its
 * own — see its sibling AppliedCpvChips' doc comment for why, and the
 * tenders page, which renders that shared label once, above both.
 */
export function AppliedFilterChips({
  applied,
  explicit,
  locale,
  onClear,
}: {
  /** What the search actually ran with, as reported by the server. */
  applied?: TenderFilters;
  explicit: ExplicitFilterFacets;
  locale: string;
  /** Clears the query text, which is where these constraints came from. */
  onClear: () => void;
}) {
  const { t } = useTranslation();
  const entries = appliedFilterEntries(applied, explicit);
  if (entries.length === 0) return null;

  // Formatted without a currency: the bound is a plain number the parser
  // read out of the query, and stamping a currency on it would assert
  // something the query never said.
  //
  // Scaled back down because the bound travels in minor units (it is compared
  // against a minor-unit column) while the user typed — and must read back —
  // whole euros: a chip answering "sotto 100k" with "100.000.000" would look
  // like the search had misunderstood them.
  const formatValue = (v: bigint) => new Intl.NumberFormat(locale).format(v / 100n);
  const formatDate = (d: Date) =>
    new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(d);

  const chips = entries.map((e) => {
    switch (e.kind) {
      case 'country':
        return countryName(e.code, locale);
      case 'cpv':
        return t('tenders.applied.cpv', { code: e.prefix });
      case 'status':
        return t(`tenders.status.${e.status}`);
      case 'valueRange':
        return t('tenders.applied.valueRange', {
          min: formatValue(e.min),
          max: formatValue(e.max),
        });
      case 'valueMax':
        return t('tenders.applied.valueMax', { max: formatValue(e.max) });
      case 'valueMin':
        return t('tenders.applied.valueMin', { min: formatValue(e.min) });
      case 'deadlineUntil':
        return t('tenders.applied.deadlineUntil', { date: formatDate(e.date) });
      default: {
        // Unreachable: AppliedFilterEntry's kind union is exhausted above.
        // Typed as `never` so adding a new entry kind without a case here
        // fails the build, rather than silently rendering an empty chip.
        const exhaustive: never = e;
        throw new Error(`applied filter entry: unhandled kind ${JSON.stringify(exhaustive)}`);
      }
    }
  });

  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
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
