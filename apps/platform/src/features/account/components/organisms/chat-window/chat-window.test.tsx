import { render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { agentClient, workspaceClient } from '~/lib/api/client';
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
  workspaceClient: { getClientProfile: vi.fn() },
}));
const captureMock = vi.fn();
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: captureMock }) }));
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
// Rendering a tender_results message renders TenderResultCard wrapped in
// useTenderLink, which otherwise needs router/auth-store context this test
// doesn't set up — same stub message-bubble.test.tsx uses.
vi.mock('~/features/tenders', () => ({
  useTenderLink:
    () => (id: string, children: ReactNode, _className?: string, onClick?: () => void) => (
      <a href={`/tenders/${id}`} onClick={onClick}>
        {children}
      </a>
    ),
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
    captureMock.mockClear();
    useChatStore.getState().reset();
    useChatStore.setState({ draft: null, credits: null });
    useWorkspaceStore.setState({ currentWorkspaceId: 'ws-1' });
    vi.mocked(agentClient.getCredits).mockResolvedValue({
      remaining: 100,
      monthlyMax: 200,
      used: 100,
      resetDate: '',
    } as never);
    vi.mocked(workspaceClient.getClientProfile).mockResolvedValue({
      exists: true,
      profile: { sectors: ['45'], countries: ['IT'], valueMin: 0n, valueMax: 0n },
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

  it('reconstructs a tender_results history message into a rendered TenderResultCard', async () => {
    const tenders = [
      {
        id: 't-1',
        title: 'Cestini intelligenti IoT',
        buyerName: 'Comune di Torino',
        status: 'open',
        country: 'IT',
        cpv: '34928480',
        value: 250000,
        currency: 'EUR',
        deadline: '',
        source: 'ted',
      },
    ];
    vi.mocked(agentClient.getMessages).mockResolvedValue({
      messages: [
        {
          id: 'msg-1',
          sessionId: 'chat-1',
          role: 'tender_results',
          content: '',
          choices: new Uint8Array(),
          metadata: new Uint8Array(),
          tenders: new TextEncoder().encode(JSON.stringify(tenders)),
          createdAt: new Date().toISOString(),
        },
      ],
    } as never);

    useChatStore.setState({ currentChatId: 'chat-1' });

    render(<ChatWindow />);

    await waitFor(() => {
      expect(screen.getByText('Cestini intelligenti IoT')).toBeInTheDocument();
    });
  });

  it('renders a tool-chip while streaming for each active tool', async () => {
    vi.mocked(agentClient.getMessages).mockResolvedValue({ messages: [] } as never);
    useChatStore.setState({ currentChatId: 'chat-1' });

    render(<ChatWindow />);

    // Set the streaming/activeTools state *after* mount: the mount-only
    // stale-streaming-state guard only fires once, on the initial mount, so
    // updating the store post-render doesn't get wiped by it.
    useChatStore.setState({
      streaming: true,
      activeTools: [{ name: 'search_tenders', status: 'running' }],
    });

    await waitFor(() => {
      expect(screen.getByText('Cerco bandi…')).toBeInTheDocument();
    });
  });

  it('clears stale streaming state on mount', async () => {
    vi.mocked(agentClient.getMessages).mockResolvedValue({ messages: [] } as never);
    // Simulate state rehydrated as mid-stream from a prior tab / network drop.
    useChatStore.setState({
      currentChatId: 'chat-1',
      streaming: true,
      streamingContent: 'half a sentence',
      activeTools: [{ name: 'search_tenders', status: 'running' }],
    });

    render(<ChatWindow />);

    await waitFor(() => {
      expect(useChatStore.getState().streaming).toBe(false);
    });
    expect(useChatStore.getState().streamingContent).toBe('');
    expect(useChatStore.getState().activeTools).toEqual([]);
  });

  it('fires chat_session_started and client_profile_injected on the first send', async () => {
    vi.mocked(agentClient.getMessages).mockResolvedValue({ messages: [] } as never);
    useChatStore.setState({ currentChatId: 'chat-1', draft: 'Ciao' });

    render(<ChatWindow />);

    await waitFor(() => {
      expect(captureMock).toHaveBeenCalledWith(
        'chat_session_started',
        expect.objectContaining({
          location: 'explore',
          has_workspace: true,
          has_client_profile: true,
        }),
      );
    });
    expect(captureMock).toHaveBeenCalledWith(
      'client_profile_injected',
      expect.objectContaining({ location: 'explore_chat', has_sectors: true, has_countries: true }),
    );
  });
});
