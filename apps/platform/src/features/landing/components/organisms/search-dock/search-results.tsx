import { cn, Pill } from '@tendersbay/components/core';
import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { motion, useReducedMotion } from 'motion/react';
import { useTranslation } from 'react-i18next';
// Reuse the tender-feed display formatters via its barrel — the same field
// logic the account search cards use, so a tender reads identically everywhere.
import {
  countryFlag,
  countryName,
  deadlineInfo,
  formatTenderValue,
  tenderTitle,
} from '~/features/account/components/organisms/tender-feed';
import { useTenderLink } from '~/features/tenders';
import type { LandingSearchState } from './use-landing-search';

/** Shared warm-card chrome for every non-idle state (result, loading, empty, error). */
const CARD_CLASS =
  'w-full rounded-2xl border border-cream-200 bg-white/85 shadow-soft backdrop-blur';

function ResultCard({
  tender,
  index,
  reduce,
}: {
  tender: TenderResult;
  index: number;
  reduce: boolean | null;
}) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language;
  const tenderLink = useTenderLink();

  const Flag = tender.country ? countryFlag(tender.country) : null;
  const originName = tender.country ? countryName(tender.country, locale) : null;
  const { title } = tenderTitle(tender.title, tender.country);
  const value = formatTenderValue(tender.value, tender.currency, locale);

  const deadline = deadlineInfo(tender.deadline, new Date());
  const deadlineLabel = deadline
    ? deadline.days < 0
      ? t('tenders.deadline.expired')
      : deadline.days === 0
        ? t('tenders.deadline.today')
        : t('tenders.deadline.days', { count: deadline.days })
    : null;

  return (
    <motion.li
      initial={reduce ? false : { opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={
        reduce ? { duration: 0 } : { duration: 0.3, delay: index * 0.06, ease: [0.22, 1, 0.36, 1] }
      }
    >
      {tenderLink(
        tender.id,
        <>
          <div className="flex items-center gap-2">
            {Flag && (
              <span className="block w-5 shrink-0 overflow-hidden rounded ring-1 ring-ink-900/10">
                <Flag aria-hidden="true" className="block h-auto w-full" />
              </span>
            )}
            {originName && (
              <span className="min-w-0 truncate text-xs font-medium text-ink-500">
                {originName}
              </span>
            )}
            {deadline && (
              <Pill tone={deadline.tone} className="ml-auto shrink-0">
                {deadlineLabel}
              </Pill>
            )}
          </div>

          <p className="mt-1.5 line-clamp-2 text-sm font-medium leading-snug text-ink-900">
            {title}
          </p>
          {value && (
            <p className="mt-1 font-mono text-xs font-medium tabular-nums text-brand-700">
              {value}
            </p>
          )}
        </>,
        cn(
          CARD_CLASS,
          'block px-4 py-3 no-underline outline-none focus-visible:ring-2 focus-visible:ring-brand-600',
        ),
      )}
    </motion.li>
  );
}

export type SearchResultsProps = LandingSearchState & { className?: string };

/**
 * Renders the dock's search outcome under the input: up to three result cards
 * that fade + slide in (static under reduced motion), or an honest loading /
 * empty / error line. Never invents cards — an empty result says so. A polite
 * `role="status"` live region announces every transition for assistive tech.
 */
export function SearchResults({ status, results, className }: SearchResultsProps) {
  const { t } = useTranslation();
  const reduce = useReducedMotion();

  const announcement =
    status === 'loading'
      ? t('landing.search.loading')
      : status === 'error'
        ? t('landing.search.error')
        : status === 'empty'
          ? t('landing.search.empty')
          : status === 'results'
            ? t('landing.search.results', { count: results.length })
            : '';

  return (
    <div className={cn('w-full max-w-md', className)}>
      {/* Always mounted so the live region is registered before it updates. */}
      <p className="sr-only" role="status">
        {announcement}
      </p>

      {status === 'loading' && (
        <div
          aria-hidden="true"
          className={cn(
            CARD_CLASS,
            'flex items-center justify-center gap-2.5 px-4 py-3 text-sm text-ink-500',
          )}
        >
          <span className="block h-4 w-4 animate-spin rounded-full border-2 border-cream-300 border-t-brand-500 motion-reduce:animate-none" />
          {t('landing.search.loading')}
        </div>
      )}

      {status === 'empty' && (
        <div className={cn(CARD_CLASS, 'px-4 py-3 text-center text-sm text-ink-500')}>
          {t('landing.search.empty')}
        </div>
      )}

      {status === 'error' && (
        <div className={cn(CARD_CLASS, 'px-4 py-3 text-center text-sm text-ink-600')}>
          {t('landing.search.error')}
        </div>
      )}

      {status === 'results' && (
        <ul className="flex flex-col gap-2">
          {results.map((tender, index) => (
            <ResultCard key={tender.id} tender={tender} index={index} reduce={reduce} />
          ))}
        </ul>
      )}
    </div>
  );
}
