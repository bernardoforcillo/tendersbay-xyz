import type { ChatSession } from '@tendersbay/proto/agent/v1/agent_pb';
import { useCallback, useEffect, useState } from 'react';
import { agentClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

/**
 * The current user's workspace-level chat sessions, newest first. Backs the
 * "riprendi" brief card on Oggi; ListChats is workspace-scoped (no workbench
 * filter exists) and returns every member's chats, so we filter client-side
 * to the authenticated user and exclude workbench-scoped chats (promotion
 * into a workbench is one-way — a promoted chat should only resume there,
 * not leak back into the /explore resume list).
 */
export function useWorkspaceChats(workspaceId: string) {
  const userId = useAuthStore((s) => s.user?.id);
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
    setData(null);
    agentClient
      .listChats({ workspaceId })
      .then((res) => {
        if (!active) return;
        const mine = res.chats.filter((c) => c.userId === userId && c.workbenchId === '');
        const sorted = [...mine].sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : -1));
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
  }, [workspaceId, tick, userId]);

  return { data, loading, error, refetch };
}
