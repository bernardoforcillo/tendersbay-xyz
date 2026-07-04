import { useCallback, useEffect, useState } from 'react';
import { workspaceClient } from '~/lib/api/client';

/**
 * Minimal load-on-mount data hook over a stable async function, matching the
 * app's direct-RPC + local-state convention (no tanstack-query). Callers wrap
 * their RPC call in `useCallback` so identity is stable; `refetch` re-runs it.
 */
export function useAsync<T>(fn: () => Promise<T>) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const refetch = useCallback(() => setTick((n) => n + 1), []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: `tick` is a manual refetch trigger
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    fn()
      .then((res) => {
        if (active) setData(res);
      })
      .catch((e: unknown) => {
        if (active) setError(e instanceof Error ? e.message : 'Something went wrong');
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [fn, tick]);

  return { data, loading, error, refetch };
}

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
