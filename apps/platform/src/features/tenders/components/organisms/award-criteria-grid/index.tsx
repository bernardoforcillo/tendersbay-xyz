import { Card, cn, Pill } from '@tendersbay/components/core';
import { ExternalLink } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { useCaptureEventOnce } from '~/analytics';
import type { AwardCriterionView, TenderCriteriaSource } from '~/features/tenders/constants';

export type AwardCriteriaGridProps = {
  tender: TenderCriteriaSource;
  /** Analytics surface for `tender_criteria_viewed`. */
  location: AnalyticsLocation;
  className?: string;
};

/**
 * Where the points are won.
 *
 * Three things this component refuses to do, each because the honest answer and
 * the convenient answer differ:
 *
 *  1. **It never renders a weight of 0 for an unpublished weight.** `weightSet`
 *     gates the cell. A zero weight and an absent weight are different published
 *     facts, and flattening them makes "criteria published" read as "grid
 *     recoverable" — which overstates real coverage by roughly 2x.
 *  2. **It distinguishes three empty-ish states with three different strings.**
 *     "The notice names the award method but publishes no weights" (the measured
 *     majority of Italian notices, and the product's core claim about itself),
 *     "we have not read this notice's structured data yet", and a grid that is
 *     genuinely empty. "No criteria" for any of them would be a lie.
 *  3. **It collapses, it does not repeat.** Multilingual notices repeat every
 *     text field once per language, and a 13-lot notice usually publishes one
 *     grid that applies to all 13. Rendering the raw list gives the reader the
 *     same table two or three times over.
 */
export function AwardCriteriaGrid({ tender, location, className }: AwardCriteriaGridProps) {
  const { t, i18n } = useTranslation();

  const all = tender.criteria ?? [];
  const visible = filterToOneLanguage(all, i18n.language, tender.language);
  const groups = groupByLot(visible);
  const collapsed = collapseIdenticalLots(groups);

  // Keyed on the SNAPSHOT — how many criteria this notice yielded and when it
  // was enriched — so a re-render reports nothing new and a re-enrichment does.
  useCaptureEventOnce('tender_criteria_viewed', `${visible.length}:${tender.enrichedAt ?? ''}`, {
    location,
    criteria_count: visible.length,
    weighted_criteria_count: visible.filter((c) => c.weightSet).length,
    // null, not false: "never enriched" is a third state, and a boolean would
    // record it as "the notice has no usable grid" — a claim we have not earned.
    // The catalogue types this as `tri_bool` for exactly that reason, so the
    // null survives the scrubber instead of being coerced to false.
    grid_usable: tender.gridUsableSet ? Boolean(tender.gridUsable) : null,
    lot_count: groups.length,
  });

  return (
    <section aria-labelledby="award-criteria-title" className={cn('space-y-3', className)}>
      <h2 id="award-criteria-title" className="text-sm font-semibold text-ink-900">
        {t('tenders.criteria.title', 'How the points are awarded')}
      </h2>

      {visible.length === 0 ? (
        <EmptyCriteria tender={tender} />
      ) : (
        <div className="flex flex-col gap-4">
          {collapsed.map((group) => (
            <Card key={group.key} padding="none" className="overflow-hidden">
              <p className="border-b border-cream-200 px-4 py-2 text-xs font-semibold uppercase tracking-wide text-ink-400">
                {group.appliesToEveryLot
                  ? t('tenders.criteria.appliesToEveryLot', 'Applies to every lot')
                  : t('tenders.criteria.lot', { lot: group.lotRef, defaultValue: 'Lot {{lot}}' })}
              </p>
              <ul className="divide-y divide-cream-200">
                {group.criteria.map((criterion, i) => (
                  <li
                    key={`${group.key}-${criterion.ordinal ?? i}-${criterion.name}`}
                    className="flex items-start justify-between gap-4 px-4 py-3"
                  >
                    <div className="min-w-0">
                      <p className="text-sm text-ink-900">{criterion.name}</p>
                      {criterion.description && (
                        <p className="mt-1 text-xs text-ink-500">{criterion.description}</p>
                      )}
                      <Pill tone="neutral" className="mt-2">
                        {t(`tenders.criteria.type.${criterion.type}`, criterion.type)}
                      </Pill>
                    </div>
                    <Weight criterion={criterion} />
                  </li>
                ))}
              </ul>
            </Card>
          ))}
        </div>
      )}
    </section>
  );
}

function Weight({ criterion }: { criterion: AwardCriterionView }) {
  const { t, i18n } = useTranslation();

  if (!criterion.weightSet || criterion.weight === undefined) {
    return (
      <span className="shrink-0 text-xs text-ink-400 grayscale">
        {t('tenders.criteria.noWeight', 'No weight published')}
      </span>
    );
  }

  const parsed = new Intl.NumberFormat(i18n.language).format(criterion.weight);
  const raw = criterion.weightRaw?.trim();
  // The published form is kept beside the parsed one when they differ ("30%" vs
  // 30), so a parse that loses the unit loses comparability, not the datum.
  const showRaw = Boolean(raw) && raw !== parsed && raw !== String(criterion.weight);

  return (
    <span className="shrink-0 text-right">
      <span className="text-sm font-semibold text-ink-900">{parsed}</span>
      {showRaw && <span className="ml-1 text-xs text-ink-400">({raw})</span>}
      <span className="sr-only"> {t('tenders.criteria.weight', 'Weight')}</span>
    </span>
  );
}

function EmptyCriteria({ tender }: { tender: TenderCriteriaSource }) {
  const { t } = useTranslation();

  // gridUsableSet === false means the notice was never enriched. That is not
  // "the grid is unusable" — it is "we have not looked", and saying the first
  // when the second is true is the failure mode this whole phase exists to fix.
  if (!tender.gridUsableSet) {
    return (
      <Card className="bg-cream-50">
        <p className="text-sm text-ink-500 grayscale">
          {t('tenders.criteria.notEnriched', 'We have not read this notice’s structured data yet.')}
        </p>
      </Card>
    );
  }

  return (
    <Card className="bg-cream-50">
      <p className="text-sm text-ink-700">
        {t(
          'tenders.criteria.gridUnusable',
          'The notice names the award method but publishes no weights. The scoring grid is in the disciplinare.',
        )}
      </p>
      {tender.documentsUrl && (
        <a
          href={tender.documentsUrl}
          target="_blank"
          rel="noopener noreferrer nofollow"
          className="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-brand-700 underline underline-offset-2"
        >
          {t('tenders.coverage.openBuyerDocuments', 'Open the buyer’s documents')}
          <ExternalLink aria-hidden="true" strokeWidth={1.75} className="h-3 w-3 shrink-0" />
        </a>
      )}
    </Card>
  );
}

type LotGroup = {
  key: string;
  lotRef: string;
  appliesToEveryLot: boolean;
  criteria: AwardCriterionView[];
};

/**
 * ISO 639-1 (what `i18n.language` carries) → ISO 639-2/T (what eForms publishes),
 * for the 24 official EU languages. Only the terminological codes appear in
 * notices, so no bibliographic aliases (`ger`, `fre`, `dut`…) are needed.
 */
const ISO_639_2T: Record<string, string> = {
  bg: 'BUL',
  cs: 'CES',
  da: 'DAN',
  de: 'DEU',
  el: 'ELL',
  en: 'ENG',
  es: 'SPA',
  et: 'EST',
  fi: 'FIN',
  fr: 'FRA',
  ga: 'GLE',
  hr: 'HRV',
  hu: 'HUN',
  it: 'ITA',
  lt: 'LIT',
  lv: 'LAV',
  mt: 'MLT',
  nl: 'NLD',
  pl: 'POL',
  pt: 'POR',
  ro: 'RON',
  sk: 'SLK',
  sl: 'SLV',
  sv: 'SWE',
};

/**
 * A multilingual notice carries the same criterion once per published language.
 * Pick one: the reader's UI language, else the tender's own, else whichever came
 * first. When only one language is present (or none is tagged) nothing is
 * filtered — an untagged grid must not vanish.
 */
export function filterToOneLanguage(
  criteria: AwardCriterionView[],
  uiLocale: string,
  tenderLanguage?: string,
): AwardCriterionView[] {
  const present = new Set(
    criteria.map((c) => (c.lang ?? '').toUpperCase()).filter((l) => l.length > 0),
  );
  if (present.size <= 1) return criteria;

  const uiTwo = uiLocale.split('-')[0]?.toLowerCase() ?? '';
  const preferences = [ISO_639_2T[uiTwo], tenderLanguage?.toUpperCase()].filter((l): l is string =>
    Boolean(l),
  );
  const chosen =
    preferences.find((l) => present.has(l)) ??
    criteria.find((c) => c.lang)?.lang?.toUpperCase() ??
    '';
  if (!chosen) return criteria;
  return criteria.filter((c) => (c.lang ?? '').toUpperCase() === chosen);
}

/** Groups by lot in encounter order — the server already sorts by (lot_ref, ordinal). */
function groupByLot(criteria: AwardCriterionView[]): LotGroup[] {
  const byLot = new Map<string, AwardCriterionView[]>();
  for (const criterion of criteria) {
    const lot = criterion.lotRef ?? '';
    const list = byLot.get(lot);
    if (list) list.push(criterion);
    else byLot.set(lot, [criterion]);
  }
  return [...byLot.entries()].map(([lotRef, list]) => ({
    key: lotRef || '__notice__',
    lotRef,
    // "" is notice-level: it applies to every lot by definition.
    appliesToEveryLot: lotRef === '',
    criteria: list,
  }));
}

/**
 * Collapses per-lot groups that publish an identical grid into one block, the
 * same collapse the agent payload already performs. A 13-lot notice sharing one
 * grid is the common case, not a curiosity.
 */
function collapseIdenticalLots(groups: LotGroup[]): LotGroup[] {
  if (groups.length < 2) return groups;
  const [first, ...rest] = groups;
  if (!first) return groups;
  const signature = signatureOf(first.criteria);
  if (!rest.every((g) => signatureOf(g.criteria) === signature)) return groups;
  return [{ ...first, key: '__all-lots__', appliesToEveryLot: true }];
}

function signatureOf(criteria: AwardCriterionView[]): string {
  return JSON.stringify(
    criteria.map((c) => [c.type, c.name, c.weightSet ? c.weight : null, c.weightRaw ?? '']),
  );
}
