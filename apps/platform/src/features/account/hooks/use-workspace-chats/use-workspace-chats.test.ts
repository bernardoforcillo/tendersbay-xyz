import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listChats = vi.fn();
vi.mock('~/lib/api/client', () => ({
  agentClient: { listChats: (...args: unknown[]) => listChats(...args) },
}));

import { useAuthStore } from '~/store/auth';
import { useWorkspaceChats } from './index';

describe('useWorkspaceChats', () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: { id: 'u-1', email: '', displayName: '' },
      accessToken: null,
      isAuthenticated: true,
    });
  });

  it('loads only the current user chats for the workspace, newest first', async () => {
    listChats.mockResolvedValue({
      chats: [
        {
          id: 'a',
          userId: 'u-1',
          workbenchId: '',
          title: 'Old',
          updatedAt: '2026-07-01T10:00:00Z',
        },
        {
          id: 'b',
          userId: 'u-1',
          workbenchId: '',
          title: 'New',
          updatedAt: '2026-07-12T10:00:00Z',
        },
        {
          id: 'c-other',
          userId: 'u-2',
          workbenchId: '',
          title: 'Not mine',
          updatedAt: '2026-07-13T10:00:00Z',
        },
      ],
    });
    const { result } = renderHook(() => useWorkspaceChats('ws-1'));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(listChats).toHaveBeenCalledWith({ workspaceId: 'ws-1' });
    expect(result.current.data?.map((c) => c.id)).toEqual(['b', 'a']);
  });

  it('excludes workbench-scoped chats from the explore resume list', async () => {
    listChats.mockResolvedValue({
      chats: [
        { id: 'ws-chat', userId: 'u-1', workbenchId: '', updatedAt: '2026-07-02T00:00:00Z' },
        { id: 'wb-chat', userId: 'u-1', workbenchId: 'wb1', updatedAt: '2026-07-03T00:00:00Z' },
      ],
    });
    const { result } = renderHook(() => useWorkspaceChats('ws-1'));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data?.map((c) => c.id)).toEqual(['ws-chat']);
  });
});
