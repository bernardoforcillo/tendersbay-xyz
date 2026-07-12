import type { ChatSession } from '@tendersbay/proto/agent/v1/agent_pb';
import { useCallback, useEffect, useState } from 'react';
import { agentClient } from '~/lib/api/client';

/**
 * The workspace's chat sessions, newest first. Backs the "riprendi" brief
 * card on Oggi; ListChats is workspace-scoped (no workbench filter exists).
 */
export function useWorkspaceChats(workspaceId: string) {
  const [data, setData] = useState<ChatSession[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const refetch = useCallback(() => setTick((n) => n + 1), []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: tick is intentionally included to enable manual refetch
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    agentClient
      .listChats({ workspaceId })
      .then((res) => {
        if (!active) return;
        const sorted = [...res.chats].sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : -1));
        setData(sorted);
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
  }, [workspaceId, tick]);

  return { data, loading, error, refetch };
}
