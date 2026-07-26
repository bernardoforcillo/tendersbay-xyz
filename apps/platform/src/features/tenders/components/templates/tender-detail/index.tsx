import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import type { ReactNode } from 'react';
import type { TenderDetailData } from '../../../load-tender-detail';
import {
  RelatedTenders,
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
};

export function TenderDetailView({
  tender,
  related,
  renderRelated,
  actions,
}: TenderDetailViewProps) {
  return (
    <article className="mx-auto w-full max-w-2xl space-y-8 py-6">
      <TenderDetailHeader tender={tender} actions={actions} />
      <TenderDetailMeta tender={tender} />
      <TenderLots tender={tender} />
      <TenderDocuments tender={tender} />
      <RelatedTenders related={related} renderItem={renderRelated} />
    </article>
  );
}
