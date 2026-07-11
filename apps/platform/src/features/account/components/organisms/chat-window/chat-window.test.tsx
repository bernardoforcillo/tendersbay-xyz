import { render, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { agentClient } from '~/lib/api/client';
import { useChatStore } from '~/store/chat';
import { useWorkspaceStore } from '~/store/workspace';

const { sendMessageMock } = vi.hoisted(() => ({ sendMessageMock: vi.fn() }));

vi.mock('~/lib/api/client', () => ({
  agentClient: {
    getMessages: vi.fn(),
    createChat: vi.fn(),
    getCredits: vi.fn(),
    chatStream: vi.fn(),
  },
}));
vi.mock('~/features/account/hooks/use-chat-stream', () => ({
  useChatStream: () => ({ sendMessage: sendMessageMock, submitChoice: vi.fn(), cancel: vi.fn() }),
}));
// Return defaultValue strings without initializing the full i18n stack.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, defaultValue?: string | { defaultValue?: string }) =>
      typeof defaultValue === 'string' ? defaultValue : (defaultValue?.defaultValue ?? key),
    i18n: { language: 'en-ie' },
  }),
}));

import { ChatWindow } from './index';

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

describe('ChatWindow', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useChatStore.getState().reset();
    useChatStore.setState({ draft: null, credits: null });
    useWorkspaceStore.setState({ currentWorkspaceId: 'ws-1' });
    vi.mocked(agentClient.getCredits).mockResolvedValue({
      remaining: 100,
      monthlyMax: 200,
      used: 100,
      resetDate: '',
    } as never);
  });

  it('waits for history to settle before sending a pre-set draft, then sends it', async () => {
    const deferred = createDeferred<{ messages: never[] }>();
    vi.mocked(agentClient.getMessages).mockReturnValue(deferred.promise as never);

    useChatStore.setState({ currentChatId: 'chat-1', draft: 'Hello agent' });

    render(<ChatWindow />);

    // getMessages is still in flight — the draft must not be sent yet.
    expect(sendMessageMock).not.toHaveBeenCalled();

    deferred.resolve({ messages: [] });

    await waitFor(() => {
      expect(sendMessageMock).toHaveBeenCalledWith('chat-1', 'Hello agent');
    });
    expect(useChatStore.getState().draft).toBeNull();
  });
});
