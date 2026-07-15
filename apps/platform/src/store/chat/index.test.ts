import { beforeEach, describe, expect, it } from 'vitest';
import { useChatStore } from './index';

describe('useChatStore.addMessage', () => {
  beforeEach(() => {
    useChatStore.getState().reset();
  });

  it('appends a new message', () => {
    useChatStore.getState().addMessage({
      id: 'msg-1',
      role: 'user',
      content: 'Hello',
      createdAt: new Date().toISOString(),
    });
    expect(useChatStore.getState().messages).toHaveLength(1);
  });

  it('ignores a message whose id is already present, instead of duplicating it', () => {
    const msg = {
      id: 'msg-1',
      role: 'user' as const,
      content: 'Hello',
      createdAt: new Date().toISOString(),
    };
    useChatStore.getState().addMessage(msg);
    useChatStore.getState().addMessage(msg);
    expect(useChatStore.getState().messages).toHaveLength(1);
  });

  it('still appends a different message id after an ignored duplicate', () => {
    const store = useChatStore.getState();
    store.addMessage({
      id: 'msg-1',
      role: 'user',
      content: 'Hello',
      createdAt: new Date().toISOString(),
    });
    store.addMessage({
      id: 'msg-1',
      role: 'user',
      content: 'Hello',
      createdAt: new Date().toISOString(),
    });
    store.addMessage({
      id: 'msg-2',
      role: 'assistant',
      content: 'Hi there',
      createdAt: new Date().toISOString(),
    });
    expect(useChatStore.getState().messages.map((m) => m.id)).toEqual(['msg-1', 'msg-2']);
  });
});
