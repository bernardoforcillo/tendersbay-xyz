import { beforeEach, describe, expect, it } from 'vitest';
import { useFirstRunStore } from './index';

describe('useFirstRunStore', () => {
  beforeEach(() => {
    useFirstRunStore.setState({ skippedWorkspaceIds: [] });
  });

  it('has no skipped workspaces by default', () => {
    expect(useFirstRunStore.getState().hasSkipped('ws-1')).toBe(false);
  });

  it('records a skipped workspace', () => {
    useFirstRunStore.getState().skipWorkspace('ws-1');
    expect(useFirstRunStore.getState().hasSkipped('ws-1')).toBe(true);
    expect(useFirstRunStore.getState().skippedWorkspaceIds).toEqual(['ws-1']);
  });

  it('is per-workspace — skipping one workspace does not skip another', () => {
    useFirstRunStore.getState().skipWorkspace('ws-1');
    expect(useFirstRunStore.getState().hasSkipped('ws-2')).toBe(false);
  });

  it('does not duplicate an already-skipped workspace id', () => {
    useFirstRunStore.getState().skipWorkspace('ws-1');
    useFirstRunStore.getState().skipWorkspace('ws-1');
    expect(useFirstRunStore.getState().skippedWorkspaceIds).toEqual(['ws-1']);
  });
});
