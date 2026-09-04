import { Banner, Card, cn } from '@tendersbay/components/core';
import type { Gap, GetResponsePreviewResponse } from '@tendersbay/proto/espd/v1/espd_pb';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { useCaptureEventOnce } from '~/analytics';
import { DgueGapRow } from '~/features/espd/components/molecules/dgue-gap-row';

export type DgueReadinessCardProps = {
  preview: GetResponsePreviewResponse;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Open the dossier (company-scoped gaps) or the per-gara form (bid-scoped). */
  onOpenGap: (gap: Gap) => void;
  /** Scroll to / expand the re-confirmation screen. */
  onReconfirm: () => void;
  /** Scroll to / expand the export sheet. Only offered once the document is ready. */
  onExport: () => void;
  className?: string;
};

/**
 * "Your DGUE: 31 of 34 fields ready" — the card that replaces the status-and-note
 * checklist.
 *
 * Three deliberate choices about how this reads:
 *
 *  - **It leads with what is DONE.** Endowed progress: a person who arrives at a
 *    dossier they have never filled still sees a bar with something in it,
 *    because the identity fields they typed at sign-up are already in the
 *    document. The old checklist opened at zero of fourteen and asked for
 *    fourteen decisions.
 *  - **Two to-do lists, never one.** A company gap is answered ONCE and closes
 *    for every future gara; a bid gap belongs to this one. Merging them would
 *    hide the compounding half of the product's proposition behind a flat
 *    count, and would ask a person to re-answer for gara two what they answered
 *    for gara one.
 *  - **Never a red wall.** No gap is styled as an error, because none of them is
 *    a failure — they are the questions nobody has been asked yet. The banner
 *    only turns celebratory at zero, which is the one state worth marking.
 */
export function DgueReadinessCard({
  preview,
  location,
  onOpenGap,
  onReconfirm,
  onExport,
  className,
}: DgueReadinessCardProps) {
  const { t } = useTranslation();

  const companyGaps = preview.gaps.filter((g) => g.scope === 'company');
  const bidGaps = preview.gaps.filter((g) => g.scope === 'bid');
  const declarations = preview.declarations;
  const total = preview.readyCount + preview.missingCount;
  const pct = total === 0 ? 0 : Math.round((preview.readyCount / total) * 100);

  // Keyed by the composed snapshot, so the deliberate refetch after every
  // captured answer does not fire this again and inflate the denominator of
  // every rate computed from it.
  useCaptureEventOnce('dgue_readiness_viewed', preview.composedAt || null, {
    location,
    ready: preview.ready,
    ready_field_count: preview.readyCount,
    gap_count: preview.gaps.length,
    company_gap_count: companyGaps.length,
    bid_gap_count: bidGaps.length,
    declarations_complete: declarations?.complete ?? false,
    declarations_confirmed: declarations?.confirmed ?? false,
    request_known: preview.requestKnown,
  });

  return (
    <Card className={className}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="font-semibold text-ink-700 text-sm">
          {t('espd.readiness.title', 'Your DGUE')}
        </h2>
        <span className="text-ink-500 text-xs">
          {t('espd.readiness.progress', {
            ready: preview.readyCount,
            total,
            defaultValue: '{{ready}} of {{total}} fields ready',
          })}
        </span>
      </div>

      {/* The bar is decorative: the numbers above already say it, and a
          progressbar role would make a screen reader read the same fact twice. */}
      <div aria-hidden="true" className="mt-3 h-2 w-full overflow-hidden rounded-full bg-cream-200">
        <div
          className="h-full rounded-full bg-brand-600 transition-[width] duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>

      {preview.ready ? (
        <Banner tone="success" className="mt-4">
          {t('espd.readiness.ready', 'Ready to export. Nothing is missing.')}
        </Banner>
      ) : (
        <div className="mt-4 flex flex-col gap-4">
          <GapList
            title={t('espd.readiness.companyTodo', 'Fix once — closes for every tender')}
            hint={t(
              'espd.readiness.companyTodoHint',
              'These are facts about your company. Answer them once and they carry to the next tender.',
            )}
            gaps={companyGaps}
            location={location}
            onOpenGap={onOpenGap}
          />
          <GapList
            title={t('espd.readiness.bidTodo', 'For this tender')}
            gaps={bidGaps}
            location={location}
            onOpenGap={onOpenGap}
          />
        </div>
      )}

      <div className="mt-4 flex flex-wrap gap-2">
        {/* The re-confirmation is offered whenever the answers exist, ready or
            not: it is the step people forget, and hiding it until everything
            else is done would make it the last surprise before a deadline. */}
        {declarations?.complete && (
          <button type="button" onClick={onReconfirm} className={SECONDARY_BUTTON}>
            {declarations.confirmed
              ? t('espd.readiness.reviewDeclarations', 'Review your declarations')
              : t('espd.readiness.confirmDeclarations', 'Confirm your declarations')}
          </button>
        )}
        <button
          type="button"
          onClick={onExport}
          aria-disabled={preview.ready ? undefined : 'true'}
          className={cn(PRIMARY_BUTTON, !preview.ready && 'cursor-default opacity-50')}
        >
          {t('espd.readiness.export', 'Export')}
        </button>
      </div>
      {!preview.ready && (
        <p className="mt-2 text-ink-400 text-xs">
          {t('espd.readiness.exportHint', 'The export unlocks when nothing is missing.')}
        </p>
      )}
    </Card>
  );
}

/**
 * One to-do list. An empty list renders its own "nothing here" line rather than
 * disappearing: a heading that vanishes makes the reader wonder whether the
 * question was asked at all.
 */
function GapList({
  title,
  hint,
  gaps,
  location,
  onOpenGap,
}: {
  title: string;
  hint?: string;
  gaps: Gap[];
  location: AnalyticsLocation;
  onOpenGap: (gap: Gap) => void;
}) {
  const { t } = useTranslation();
  return (
    <div>
      <p className="font-semibold text-ink-700 text-xs uppercase tracking-wide">{title}</p>
      {hint && <p className="mt-1 text-ink-400 text-xs">{hint}</p>}
      {gaps.length === 0 ? (
        <p className="mt-2 text-ink-500 text-sm">
          {t('espd.readiness.noneLeft', 'Nothing left here.')}
        </p>
      ) : (
        <ul className="mt-2 list-none p-0">
          {gaps.map((gap) => (
            <DgueGapRow
              key={`${gap.criterion}:${gap.field}`}
              gap={gap}
              location={location}
              onOpen={onOpenGap}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

const PRIMARY_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl bg-brand-600 px-4 font-semibold text-sm text-white ' +
  'outline-none transition-colors duration-150 hover:bg-brand-700 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2';

const SECONDARY_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl border border-cream-300 px-4 font-semibold text-ink-700 text-sm ' +
  'outline-none transition-colors duration-150 hover:bg-cream-200 focus-visible:ring-2 focus-visible:ring-brand-600';
