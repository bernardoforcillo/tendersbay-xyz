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
});
