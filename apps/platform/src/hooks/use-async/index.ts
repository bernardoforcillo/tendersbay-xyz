import { useCallback, useEffect, useState } from 'react';

/**
 * Minimal load-on-mount data hook over a stable async function, matching the
 * app's direct-RPC + local-state convention (no tanstack-query). Callers wrap
 * their RPC call in `useCallback` so identity is stable; `refetch` re-runs it.
 *
 * WHY this lives in top-level `src/hooks/` rather than inside a feature: it
 * renders nothing and knows nothing about any domain — it is cross-cutting,
 * non-UI plumbing, which @.claude/rules/frontend.md places outside `features/`
 * for exactly this reason. It previously existed as two byte-identical copies,
 * in `features/workspace/hooks` and `features/workbench/hooks`, and three other
 * features had begun importing the workbench one. That is the failure mode the
 * placement rule prevents: a feature slice that half the app depends on is a
 * shared library wearing a feature's name, and the two copies were already free
 * to drift apart on the one behaviour every caller relies on — the `active`
 * guard that stops a resolved promise writing into an unmounted component.
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
