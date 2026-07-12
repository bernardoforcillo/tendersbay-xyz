import type { ChatSession } from '@tendersbay/proto/agent/v1/agent_pb';
import { useCallback, useEffect, useState } from 'react';
import { agentClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

/**
 * The current user's chat sessions in the workspace, newest first. Backs the
 * "riprendi" brief card on Oggi; ListChats is workspace-scoped (no workbench
 * filter exists) and returns every member's chats, so we filter to the
 * authenticated user client-side.
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
        const mine = res.chats.filter((c) => c.userId === userId);
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
