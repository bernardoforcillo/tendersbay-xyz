import { useCallback, useEffect, useState } from 'react';
import { workbenchClient } from '~/lib/api/client';

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
