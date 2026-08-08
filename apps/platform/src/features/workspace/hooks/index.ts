import { useCallback } from 'react';
import { useAsync } from '~/hooks';
import { workspaceClient } from '~/lib/api/client';

// Re-exported so the two pages already importing it from here keep one import
// path. The hook itself is app-wide infra, not a workspace concern — see
// `~/hooks/use-async`.
export { useAsync };

export function useMyWorkspaces() {
  const fn = useCallback(() => workspaceClient.listMyWorkspaces({}).then((r) => r.workspaces), []);
  return useAsync(fn);
}

export function useWorkspace(workspaceId: string) {
  const fn = useCallback(
    () =>
      workspaceClient.getWorkspace({ workspaceId }).then((r) => ({
        workspace: r.workspace,
        myPermissions: r.myPermissions,
      })),
    [workspaceId],
  );
  return useAsync(fn);
}

export function useMembers(workspaceId: string) {
  const fn = useCallback(
    () => workspaceClient.listMembers({ workspaceId }).then((r) => r.members),
    [workspaceId],
  );
  return useAsync(fn);
}

export function useRoles(workspaceId: string) {
  const fn = useCallback(
    () => workspaceClient.listRoles({ workspaceId }).then((r) => r.roles),
    [workspaceId],
  );
  return useAsync(fn);
}

export function useEmailInvites(workspaceId: string) {
  const fn = useCallback(
    () => workspaceClient.listEmailInvitations({ workspaceId }).then((r) => r.invitations),
    [workspaceId],
  );
  return useAsync(fn);
}

export function useInviteLinks(workspaceId: string) {
  const fn = useCallback(
    () => workspaceClient.listInviteLinks({ workspaceId }).then((r) => r.links),
    [workspaceId],
  );
  return useAsync(fn);
}
