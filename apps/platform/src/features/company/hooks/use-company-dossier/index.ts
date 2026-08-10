import type { CompanyDossier } from '@tendersbay/proto/company/v1/company_pb';
import { useCallback } from 'react';
import { useAsync } from '~/hooks';
import { companyClient } from '~/lib/api/client';

export type CompanyDossierResult = {
  dossier: CompanyDossier | null;
  /**
   * False when nothing has ever been asserted about this company. Separate from
   * a null dossier because "we hold nothing" renders differently from "we hold
   * an empty identity" — the second is a human having cleared a field.
   */
  exists: boolean;
};

/**
 * The workspace's company dossier — backend-synced; the correction form seeds
 * local `useState` from it and owns its own draft, exactly as the client-profile
 * form does. Nothing about the dossier is cached in a store: it is server data
 * whose staleness would show up as a wrong eligibility verdict.
 */
export function useCompanyDossier(workspaceId: string) {
  const fn = useCallback(
    (): Promise<CompanyDossierResult> =>
      companyClient
        .getCompanyDossier({ workspaceId })
        .then((r) => ({ dossier: r.dossier ?? null, exists: r.exists })),
    [workspaceId],
  );
  return useAsync(fn);
}
