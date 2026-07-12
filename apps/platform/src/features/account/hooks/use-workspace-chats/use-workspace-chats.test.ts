import { renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

const listChats = vi.fn();
vi.mock('~/lib/api/client', () => ({
  agentClient: { listChats: (...args: unknown[]) => listChats(...args) },
}));

import { useWorkspaceChats } from './index';

describe('useWorkspaceChats', () => {
  it('loads chats for the workspace, newest first', async () => {
    listChats.mockResolvedValue({
      chats: [
        { id: 'a', title: 'Old', updatedAt: '2026-07-01T10:00:00Z' },
        { id: 'b', title: 'New', updatedAt: '2026-07-12T10:00:00Z' },
      ],
    });
    const { result } = renderHook(() => useWorkspaceChats('ws-1'));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(listChats).toHaveBeenCalledWith({ workspaceId: 'ws-1' });
    expect(result.current.data?.map((c) => c.id)).toEqual(['b', 'a']);
  });
});
