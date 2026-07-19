import { renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { agentClient } from '~/lib/api/client';
import { useChatStore } from '~/store/chat';
import { useChatStream } from './use-chat-stream';

vi.mock('~/lib/api/client', () => ({
  agentClient: {
    chatStream: vi.fn(),
    submitChoice: vi.fn(),
  },
}));

async function* toAsyncIterable<T>(items: T[]) {
  for (const item of items) yield item;
}

describe('useChatStream', () => {
  beforeEach(() => {
    useChatStore.getState().reset();
    vi.clearAllMocks();
  });

  it('stores a pendingChoice and stops streaming when a choice event arrives', async () => {
    vi.mocked(agentClient.chatStream).mockReturnValue(
      toAsyncIterable([
        {
          event: {
            case: 'choice',
            value: {
              id: 'choice-1',
              question: 'Private or shared?',
              options: [
                { key: 'A', label: 'Private', description: '' },
                { key: 'B', label: 'Shared', description: '' },
              ],
              allowCustom: false,
            },
          },
        },
        {
          event: {
            case: 'done',
            value: { usage: undefined, creditsRemaining: 100n, creditsMonthlyMax: 200n },
          },
        },
      ]) as never,
    );

    const { result } = renderHook(() => useChatStream());
    await result.current.sendMessage('chat-1', 'Crea un workbench');

    const state = useChatStore.getState();
    expect(state.streaming).toBe(false);
    expect(state.pendingChoice).toEqual({
      id: 'choice-1',
      question: 'Private or shared?',
      options: [
        { key: 'A', label: 'Private', description: '' },
        { key: 'B', label: 'Shared', description: '' },
      ],
      allowCustom: false,
    });
    expect(state.messages).toContainEqual(
      expect.objectContaining({
        id: 'choice-1',
        role: 'choice_prompt',
        content: 'Private or shared?',
        choices: [
          { key: 'A', label: 'Private', description: '' },
          { key: 'B', label: 'Shared', description: '' },
        ],
      }),
    );
  });

  it('submitChoice clears pendingChoice and streams the continuation through the same handling as sendMessage', async () => {
    useChatStore.getState().setPendingChoice({
      id: 'choice-1',
      question: 'Private or shared?',
      options: [{ key: 'A', label: 'Private', description: '' }],
      allowCustom: false,
    });
    vi.mocked(agentClient.submitChoice).mockReturnValue(
      toAsyncIterable([
        { event: { case: 'token', value: 'Fatto!' } },
        {
          event: {
            case: 'done',
            value: { usage: undefined, creditsRemaining: 99n, creditsMonthlyMax: 200n },
          },
        },
      ]) as never,
    );

    const { result } = renderHook(() => useChatStream());
    const credits = await result.current.submitChoice('choice-1', 'A');

    expect(agentClient.submitChoice).toHaveBeenCalledWith(
      { choiceId: 'choice-1', selectedKey: 'A', customValue: '' },
      expect.anything(),
    );
    expect(useChatStore.getState().pendingChoice).toBeNull();
    expect(credits?.remaining).toBe(99);
  });

  it('adds a tender_results message without ending the stream, then keeps concatenating tokens', async () => {
    vi.mocked(agentClient.chatStream).mockReturnValue(
      toAsyncIterable([
        { event: { case: 'token', value: 'Ho trovato ' } },
        {
          event: {
            case: 'tenderResults',
            value: {
              tenders: [
                {
                  id: 't-1',
                  title: 'Cestini intelligenti IoT',
                  buyerName: 'Comune di Torino',
                  status: 'open',
                  procedureType: 'open',
                  country: 'IT',
                  cpv: '34928480',
                  value: 250000n,
                  currency: 'EUR',
                  publishedAt: '',
                  deadline: '',
                  relevanceScore: 0,
                  source: 'ted',
                  sourceRef: 'ref-1',
                  sourceUrl: '',
                },
              ],
            },
          },
        },
        { event: { case: 'token', value: 'un risultato.' } },
        {
          event: {
            case: 'done',
            value: { usage: undefined, creditsRemaining: 100n, creditsMonthlyMax: 200n },
          },
        },
      ]) as never,
    );

    const { result } = renderHook(() => useChatStream());
    await result.current.sendMessage('chat-1', 'cestini intelligenti');

    const state = useChatStore.getState();
    expect(state.streaming).toBe(false);
    const tenderMessage = state.messages.find((m) => m.role === 'tender_results');
    expect(tenderMessage?.tenders).toHaveLength(1);
    expect(tenderMessage?.tenders?.[0]?.id).toBe('t-1');
    expect(state.messages).toContainEqual(
      expect.objectContaining({ role: 'assistant', content: 'Ho trovato un risultato.' }),
    );
  });
});
