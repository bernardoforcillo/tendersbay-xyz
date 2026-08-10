import { useCallback } from 'react';
import type { RequirementView } from '~/features/company/constants';
import { useAsync } from '~/hooks';
import { companyClient } from '~/lib/api/client';

/**
 * The participation requirements one workspace has captured for one tender.
 *
 * Workspace-scoped and refetched after every confirm/remove: one workspace's
 * mis-extraction must not decide another's go/no-go, and a confirmation is only
 * worth making if the row visibly stops asking afterwards.
 */
export function useTenderRequirements(workspaceId: string, tenderId: string) {
  const fn = useCallback(
    (): Promise<RequirementView[]> =>
      companyClient.listTenderRequirements({ workspaceId, tenderId }).then((r) => r.requirements),
    [workspaceId, tenderId],
  );
  return useAsync(fn);
}
