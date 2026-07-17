import { Code, ConnectError } from '@connectrpc/connect';
import type { TenderDetail, TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { tenderClient } from '~/lib/api/client';

export class TenderNotFoundError extends Error {}

export type TenderDetailData = { tender: TenderDetail; related: TenderResult[] };

/**
 * Loads a tender's detail + related list. A not-found tender throws
 * TenderNotFoundError (route renders its not-found state). Related failures
 * degrade to an empty list — a transport error there must not break the page.
 */
export async function loadTenderDetail(id: string): Promise<TenderDetailData> {
  let tender: TenderDetail;
  try {
    const res = await tenderClient.getTender({ id });
    if (!res.tender) throw new TenderNotFoundError(id);
    tender = res.tender;
  } catch (e) {
    if (e instanceof ConnectError && e.code === Code.NotFound) throw new TenderNotFoundError(id);
    throw e;
  }
  let related: TenderResult[] = [];
  try {
    related = (await tenderClient.getRelatedTenders({ id, limit: 6 })).results;
  } catch {
    related = [];
  }
  return { tender, related };
}
