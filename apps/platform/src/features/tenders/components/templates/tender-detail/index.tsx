import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import type { ReactNode } from 'react';
import type { AnalyticsLocation } from '~/analytics';
import type { DocumentAvailabilityView, TenderCriteriaSource } from '../../../constants';
import type { TenderDetailData } from '../../../load-tender-detail';
import {
  AwardCriteriaGrid,
  RelatedTenders,
  TenderCoverage,
  TenderDetailHeader,
  TenderDetailMeta,
  TenderDocuments,
  TenderLots,
} from '../../organisms';

export type TenderDetailViewProps = TenderDetailData & {
  /** Renders one related tender (the route supplies a typed <Link> wrapping the card). */
  renderRelated: (t: TenderResult) => ReactNode;
  /** Route-supplied header actions (authed route only — keeps the public route CTA-free). */
  actions?: ReactNode;
  /**
   * What we hold of this tender's documents.
   *
   * Route-supplied and optional for the same reason `actions` is: reading it
   * costs an authenticated, metered RPC, and the public tender page is
   * deliberately anonymous. Absent here means "not asked", so the strip renders
   * nothing at all rather than guessing a coverage it has not measured.
   */
  availability?: DocumentAvailabilityView;
  /** Analytics surface for the coverage/criteria events. */
  location?: AnalyticsLocation;
};

export function TenderDetailView({
  tender,
  related,
  renderRelated,
  actions,
  availability,
  location = 'tender_detail',
}: TenderDetailViewProps) {
  // Read the document/criteria fields through the view type rather than off the
  // generated message: it keeps "the notice was never enriched" expressible as
  // an absent field instead of forcing a zero value that would read as a fact.
  const criteriaSource: TenderCriteriaSource = tender;
  return (
    <article className="mx-auto w-full max-w-2xl space-y-8 py-6">
      <TenderDetailHeader tender={tender} actions={actions} />
      <TenderDetailMeta tender={tender} />
      <TenderLots tender={tender} />
      {availability && (
        <TenderCoverage
          availability={availability}
          documentsUrl={criteriaSource.documentsUrl}
          location={location}
        />
      )}
      <AwardCriteriaGrid tender={criteriaSource} location={location} />
      <TenderDocuments tender={tender} />
      <RelatedTenders related={related} renderItem={renderRelated} />
    </article>
  );
}
