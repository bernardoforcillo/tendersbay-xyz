import { beforeEach, describe, expect, it } from 'vitest';
import { useChatStore } from './index';

describe('chat draft (palette handoff)', () => {
  beforeEach(() => {
    useChatStore.getState().setDraft(null);
  });

  it('stores and clears a draft', () => {
    useChatStore.getState().setDraft('bandi cloud in Lombardia');
    expect(useChatStore.getState().draft).toBe('bandi cloud in Lombardia');
    useChatStore.getState().setDraft(null);
    expect(useChatStore.getState().draft).toBeNull();
  });

  it('does not persist the draft', () => {
    useChatStore.getState().setDraft('ephemeral');
    const persisted = JSON.parse(sessionStorage.getItem('chat') ?? '{}');
    expect(persisted.state?.draft).toBeUndefined();
  });
});
