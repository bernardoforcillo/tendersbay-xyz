import { Card, cn, Pill } from '@tendersbay/components/core';
import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { useTranslation } from 'react-i18next';
import {
  countryFlag,
  countryName,
  deadlineInfo,
  type FitTier,
  fitReasonFragments,
  fitTierPillClassName,
  fitTierPillTone,
  formatTenderValue,
  type ReasonSignals,
  tenderTitle,
  thresholdBadge,
} from '../tender-feed';

export type TenderResultCardProps = {
  tender: TenderResult;
  /** Set only on a per-client shortlist result (RecommendTendersForClient) — a plain search result carries neither. */
  fitTier?: FitTier;
  reason?: ReasonSignals;
  className?: string;
};

const FIT_TIER_LABEL: Record<FitTier, { key: string; defaultValue: string }> = {
  strong: { key: 'tenders.fit.tier.strong', defaultValue: 'Strong fit' },
  possible: { key: 'tenders.fit.tier.possible', defaultValue: 'Possible fit' },
  long_shot: { key: 'tenders.fit.tier.longShot', defaultValue: 'Long shot' },
};

// Fallback English text for the reason-line keys, so this component's own
// tests (and the app, briefly) render real copy before Task 17 adds these
// keys to all 24 locale files — matching this file's existing defaultValue
// convention (see the deadline labels below). "tenders.deadline.days" is
// excluded: it already exists in every locale, no fallback needed.
const REASON_DEFAULT: Record<string, string> = {
  'tenders.fit.reasonSector': 'Matches your sector',
  'tenders.fit.reasonCountry': 'In your target country',
  'tenders.fit.reasonValueInBand': 'In your value band',
  'tenders.fit.reasonValueBelow': 'Below your value band',
  'tenders.fit.reasonValueAbove': 'Above your value band',
  'tenders.fit.reasonRegion': 'In your region',
  'tenders.fit.reasonProcedure': 'Matches your procedure type',
};

// English fallback for the EU-threshold badge labels — same defaultValue convention as the
// fit/deadline labels above, so the badge reads correctly even before a locale carries the key.
const EU_THRESHOLD_DEFAULT: Record<'below' | 'above', string> = {
  below: 'Below EU threshold',
  above: 'Above EU threshold',
};

/**
 * Presentational result card for a single tender in a search feed. When `fitTier` is set (the
 * per-client shortlist), it also renders a qualitative fit Pill and a reason line built from
 * `reason` — never a numeric match %.
 *
 * Purely presentational — every call site wraps it in a `Link` to the tender's detail page
 * (see `useTenderLink`). It must never render its own `<a>`: nesting one inside that outer
 * `Link` would make the browser navigate to the inner anchor on click, silently breaking the
 * outer navigation. The source-notice URL is surfaced on the detail page instead (`TenderDocuments`).
 *
 * The header is a provenance rail: a country flag (the tender's origin) and a source stamp (the
 * portal it was published on, e.g. TED), with the deadline pill trailing. Title leads the body;
 * value, status and CPV close the card.
 */
export function TenderResultCard({ tender, fitTier, reason, className }: TenderResultCardProps) {
  const { t, i18n } = useTranslation();

  const Flag = tender.country ? countryFlag(tender.country) : null;
  const originName = tender.country ? countryName(tender.country, i18n.language) : null;

  const deadline = deadlineInfo(tender.deadline, new Date());
  const deadlineLabel = deadline
    ? deadline.days < 0
      ? t('tenders.deadline.expired', { defaultValue: 'Closed' })
      : deadline.days === 0
        ? t('tenders.deadline.today', { defaultValue: 'Closes today' })
        : t('tenders.deadline.days', {
            count: deadline.days,
            defaultValue: '{{count}} days left',
          })
    : null;

  const value = formatTenderValue(tender.value, tender.currency, i18n.language);
  const statusLabel = t(`tenders.status.${tender.status}`, { defaultValue: tender.status });
  const { title, category } = tenderTitle(tender.title, tender.country);

  // Honest EU-threshold band (a NEW axis, not the fit tier's value_fit): brand-emphasised when
  // below (SME-winnable), muted when above, and rendered NOT AT ALL when unknown/ambiguous.
  const euBadge = thresholdBadge(tender.euThreshold ?? '');

  const reasonLine = reason
    ? fitReasonFragments(reason)
        .map((f) => t(f.key, { count: f.count, defaultValue: REASON_DEFAULT[f.key] }))
        .join(' · ')
    : '';

  return (
    <Card
      padding="none"
      className={cn('p-4 transition-shadow duration-200 hover:shadow-soft-md', className)}
    >
      <div className="flex items-center gap-2">
        {Flag ? (
          <span
            title={originName ?? undefined}
            className="block w-6 shrink-0 overflow-hidden rounded ring-1 ring-ink-900/10"
          >
            <Flag aria-label={originName ?? undefined} className="block h-auto w-full" />
          </span>
        ) : (
          originName && (
            <span className="font-mono text-[10px] font-semibold uppercase tracking-wide text-ink-400">
              {originName}
            </span>
          )
        )}
        {tender.source && (
          <span className="inline-flex items-center rounded-md border border-cream-300 bg-cream-100 px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wide text-ink-500">
            {tender.source.toUpperCase()}
          </span>
        )}
        {fitTier && (
          <Pill
            tone={fitTierPillTone(fitTier)}
            className={cn('ml-auto shrink-0', fitTierPillClassName(fitTier))}
          >
            {t(FIT_TIER_LABEL[fitTier].key, { defaultValue: FIT_TIER_LABEL[fitTier].defaultValue })}
          </Pill>
        )}
        {deadline && (
          <Pill tone={deadline.tone} className={cn('shrink-0', !fitTier && 'ml-auto')}>
            {deadlineLabel}
          </Pill>
        )}
      </div>

      <p className="mt-2.5 line-clamp-2 text-sm font-medium leading-snug text-ink-900">{title}</p>
      {category && <p className="mt-1 truncate text-xs font-medium text-ink-600">{category}</p>}
      {tender.buyerName && (
        <p className="mt-0.5 truncate text-xs text-ink-500">{tender.buyerName}</p>
      )}
      {reasonLine && (
        <p data-testid="tender-fit-reason" className="mt-1.5 text-xs text-ink-500">
          {reasonLine}
        </p>
      )}

      <div className="mt-3 flex items-center justify-between gap-3 border-t border-cream-200 pt-2.5">
        <div className="flex min-w-0 items-center gap-2">
          <p className="min-w-0 truncate text-xs text-ink-500">
            <span className="font-mono font-medium tabular-nums text-brand-700">
              {value ?? t('tenders.value.unknown', { defaultValue: 'Value undisclosed' })}
            </span>
            <span className="mx-1.5 text-cream-400">·</span>
            <span>{statusLabel}</span>
          </p>
          {euBadge && (
            <Pill
              data-testid="tender-eu-threshold"
              tone={euBadge.tone === 'below' ? 'match' : 'neutral'}
              className={cn('shrink-0', euBadge.tone === 'above' && 'grayscale')}
            >
              {t(euBadge.labelKey, { defaultValue: EU_THRESHOLD_DEFAULT[euBadge.tone] })}
            </Pill>
          )}
        </div>
        {tender.cpv && (
          <span className="shrink-0 font-mono text-[10px] uppercase tracking-wide text-ink-400">
            {tender.cpv}
          </span>
        )}
      </div>
    </Card>
  );
}
