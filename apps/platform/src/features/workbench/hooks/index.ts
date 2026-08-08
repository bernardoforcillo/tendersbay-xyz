import { useCallback } from 'react';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useAsync } from '~/hooks';
import { bidClient, workbenchClient } from '~/lib/api/client';

export function useWorkbenches(workspaceId: string) {
  const fn = useCallback(
    () => workbenchClient.listWorkbenches({ workspaceId }).then((r) => r.workbenches),
    [workspaceId],
  );
  return useAsync(fn);
}

export function useWorkbench(workbenchId: string) {
  const fn = useCallback(
    () =>
      workbenchClient.getWorkbench({ workbenchId }).then((r) => ({
        workbench: r.workbench,
        myPermissions: r.myPermissions,
        workspaceName: r.workspaceName,
      })),
    [workbenchId],
  );
  return useAsync(fn);
}

export function useWorkbenchMembers(workbenchId: string) {
  const fn = useCallback(
    () => workbenchClient.listWorkbenchMembers({ workbenchId }).then((r) => r.members),
    [workbenchId],
  );
  return useAsync(fn);
}

export function useWorkbenchRoles(workbenchId: string) {
  const fn = useCallback(
    () => workbenchClient.listWorkbenchRoles({ workbenchId }).then((r) => r.roles),
    [workbenchId],
  );
  return useAsync(fn);
}

export function useBids(workbenchId: string) {
  const fn = useCallback(
    () => bidClient.listBids({ workbenchId }).then((r) => r.bids),
    [workbenchId],
  );
  return useAsync(fn);
}

export function useBid(bidId: string) {
  const { workbenchId } = useWorkbenchContext();
  const fn = useCallback(
    () => bidClient.getBid({ workbenchId, bidId }).then((r) => r.bid),
    [workbenchId, bidId],
  );
  return useAsync(fn);
}

export function useChecklist(bidId: string) {
  const { workbenchId } = useWorkbenchContext();
  const fn = useCallback(
    () => bidClient.listChecklistItems({ workbenchId, bidId }).then((r) => r.items),
    [workbenchId, bidId],
  );
  return useAsync(fn);
}
