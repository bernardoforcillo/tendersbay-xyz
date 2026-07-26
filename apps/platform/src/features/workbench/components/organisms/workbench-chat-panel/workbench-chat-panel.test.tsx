import { render, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { agentClient } from '~/lib/api/client';
import { useChatStore } from '~/store/chat';

const captureMock = vi.fn();
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: captureMock }) }));
vi.mock('~/lib/api/client', () => ({ agentClient: { listChats: vi.fn() } }));
vi.mock('~/features/workbench/context', () => ({
  useWorkbenchContext: () => ({
    workbenchId: 'wb-1',
    workbench: { id: 'wb-1', workspaceId: 'ws-1' },
  }),
}));
vi.mock('~/features/account/components/organisms', () => ({
  ChatWindow: (props: { location?: string; workbenchId?: string }) => (
    <div
      data-testid="chat-window"
      data-location={props.location}
      data-workbench={props.workbenchId}
    />
  ),
}));

import { WorkbenchChatPanel } from './index';

describe('WorkbenchChatPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    captureMock.mockClear();
    useChatStore.getState().reset();
  });

  it('resumes the most-recent workbench-scoped chat and fires workbench_chat_continued', async () => {
    vi.mocked(agentClient.listChats).mockResolvedValue({
      chats: [
        { id: 'other', workbenchId: '', updatedAt: '2026-07-20T10:00:00Z' },
        { id: 'wb-old', workbenchId: 'wb-1', updatedAt: '2026-07-19T10:00:00Z' },
        { id: 'wb-new', workbenchId: 'wb-1', updatedAt: '2026-07-21T10:00:00Z' },
      ],
    } as never);

    render(<WorkbenchChatPanel />);

    await waitFor(() => {
      expect(useChatStore.getState().currentChatId).toBe('wb-new');
    });
    expect(useChatStore.getState().currentWorkbenchId).toBe('wb-1');
    expect(agentClient.listChats).toHaveBeenCalledWith({ workspaceId: 'ws-1' });
    expect(captureMock).toHaveBeenCalledWith('workbench_chat_continued', { location: 'workbench' });
  });

  it('leaves the chat empty for a lazy create when the workbench has no chats', async () => {
    vi.mocked(agentClient.listChats).mockResolvedValue({
      chats: [{ id: 'other', workbenchId: '', updatedAt: '2026-07-20T10:00:00Z' }],
    } as never);

    render(<WorkbenchChatPanel />);

    await waitFor(() => {
      expect(useChatStore.getState().currentWorkbenchId).toBe('wb-1');
    });
    expect(useChatStore.getState().currentChatId).toBeNull();
    expect(captureMock).not.toHaveBeenCalledWith('workbench_chat_continued', expect.anything());
  });

  it('renders ChatWindow scoped to the workbench (location + workbenchId)', async () => {
    vi.mocked(agentClient.listChats).mockResolvedValue({ chats: [] } as never);

    const { getByTestId } = render(<WorkbenchChatPanel />);

    await waitFor(() => expect(getByTestId('chat-window')).toBeInTheDocument());
    expect(getByTestId('chat-window').getAttribute('data-location')).toBe('workbench');
    expect(getByTestId('chat-window').getAttribute('data-workbench')).toBe('wb-1');
  });
});
