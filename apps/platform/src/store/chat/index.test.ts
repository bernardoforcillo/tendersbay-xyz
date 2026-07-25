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

describe('chat store — ephemeral streaming/session state', () => {
  beforeEach(() => useChatStore.getState().reset());

  it('upserts activeTools by name (running → done on the same entry)', () => {
    const s = useChatStore.getState();
    s.setActiveTools('search_tenders', 'running');
    expect(useChatStore.getState().activeTools).toEqual([
      { name: 'search_tenders', status: 'running' },
    ]);
    s.setActiveTools('search_tenders', 'done');
    expect(useChatStore.getState().activeTools).toEqual([
      { name: 'search_tenders', status: 'done' },
    ]);
    s.setActiveTools('create_workbench', 'running');
    expect(useChatStore.getState().activeTools).toHaveLength(2);
  });

  it('clearActiveTools and reset() empty the ephemeral state and counters', () => {
    const s = useChatStore.getState();
    s.setActiveTools('search_tenders', 'running');
    s.incToolCalls();
    s.incTendersOpened();
    s.clearActiveTools();
    expect(useChatStore.getState().activeTools).toEqual([]);
    expect(useChatStore.getState().toolCallsTotal).toBe(1);
    // Re-populate activeTools to verify reset() actually clears it, not just clearActiveTools()
    s.setActiveTools('search_tenders', 'running');
    useChatStore.getState().reset();
    expect(useChatStore.getState().activeTools).toEqual([]);
    expect(useChatStore.getState().toolCallsTotal).toBe(0);
    expect(useChatStore.getState().tendersOpened).toBe(0);
  });

  it('excludes ephemeral fields from persistence (partialize)', () => {
    const persisted = (useChatStore.persist.getOptions().partialize as (s: unknown) => object)(
      useChatStore.getState(),
    );
    expect(persisted).not.toHaveProperty('activeTools');
    expect(persisted).not.toHaveProperty('toolCallsTotal');
    expect(persisted).not.toHaveProperty('tendersOpened');
    expect(persisted).not.toHaveProperty('streaming');
    // Assert the complete persisted shape to guard against accidental additions
    expect(persisted).toEqual({ currentChatId: null, messages: [] });
  });
});
