import { Card } from '@tendersbay/components/core';
import { ExternalLink } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation, ReportedCoverage, ReportedCoverageReason } from '~/analytics';
import { useCaptureEventOnce } from '~/analytics';
import { CoverageBadge } from '~/features/tenders/components/molecules';
import type { DocumentAvailabilityView } from '~/features/tenders/constants';

export type TenderCoverageProps = {
  availability: DocumentAvailabilityView;
  /** Buyer-side URL where the documents live — the escape hatch when we hold none. */
  documentsUrl?: string;
  /** Analytics surface for `document_coverage_shown`. */
  location: AnalyticsLocation;
  className?: string;
};

/**
 * What we have actually read about this gara, stated before anything we conclude
 * from it.
 *
 * On the scheda gara this strip sits ABOVE the verdict, deliberately: a verdict's
 * credibility is a function of what was read, and a reader who sees "notice only"
 * first calibrates every later claim correctly. Putting it below would let the
 * verdict land unqualified and then quietly footnote itself.
 *
 * It renders as a `Card`, not a `Banner`. Partial coverage is not a failure — it
 * is work not yet done, or a buyer who published nothing — and the error tone
 * would say something untrue about it.
 */
export function TenderCoverage({
  availability,
  documentsUrl,
  location,
  className,
}: TenderCoverageProps) {
  const { t } = useTranslation();

  // Once per availability SNAPSHOT, not per mount: a refetch after a captured
  // answer hands this component a new object without changing what it reports,
  // and a second event would inflate the denominator of every coverage rate.
  //
  // The casts are the wire boundary doing its job, not laxity: `coverage` and
  // `reason` are proto3 strings, so the compiler has nothing to narrow them
  // with. `analyticsEventProps` scrubs a value outside the declared vocabulary
  // to `invalid` rather than forwarding it, which turns backend drift into a
  // countable bucket instead of a silent mismatch.
  useCaptureEventOnce(
    'document_coverage_shown',
    `${availability.coverage}:${availability.reason}`,
    {
      location,
      coverage: availability.coverage as ReportedCoverage,
      reason: availability.reason as ReportedCoverageReason,
      known_document_links: availability.knownDocumentLinks ?? 0,
      extracted_documents: availability.extractedDocuments ?? 0,
    },
  );

  const counts: string[] = [];
  if ((availability.extractedDocuments ?? 0) > 0) {
    counts.push(
      t('tenders.coverage.counts.documents', {
        count: availability.extractedDocuments,
        defaultValue_one: '{{count}} document read',
        defaultValue_other: '{{count}} documents read',
      }),
    );
  }
  if ((availability.extractedParts ?? 0) > 0) {
    counts.push(
      t('tenders.coverage.counts.parts', {
        count: availability.extractedParts,
        defaultValue_one: '{{count}} passage indexed',
        defaultValue_other: '{{count}} passages indexed',
      }),
    );
  }
  if ((availability.knownDocumentLinks ?? 0) > 0) {
    counts.push(
      t('tenders.coverage.counts.knownLinks', {
        count: availability.knownDocumentLinks,
        defaultValue_one: '{{count}} document published',
        defaultValue_other: '{{count}} documents published',
      }),
    );
  }

  return (
    <Card className={className}>
      <h2 className="text-sm font-semibold text-ink-700">
        {t('tenders.coverage.title', 'What we have read')}
      </h2>
      <div className="mt-3 flex flex-col gap-2">
        <CoverageBadge coverage={availability.coverage} reason={availability.reason} />
        {counts.length > 0 && <p className="text-xs text-ink-500">{counts.join(' · ')}</p>}
        {/* Only `body_not_retrieved` earns this link: it is the one cause where the
            documents demonstrably exist and a human can read them right now. */}
        {availability.reason === 'body_not_retrieved' && documentsUrl && (
          <a
            href={documentsUrl}
            target="_blank"
            rel="noopener noreferrer nofollow"
            className="inline-flex w-fit items-center gap-1 text-xs font-semibold text-brand-700 underline underline-offset-2"
          >
            {t('tenders.coverage.openBuyerDocuments', 'Open the buyer’s documents')}
            <ExternalLink aria-hidden="true" strokeWidth={1.75} className="h-3 w-3 shrink-0" />
          </a>
        )}
      </div>
    </Card>
  );
}
