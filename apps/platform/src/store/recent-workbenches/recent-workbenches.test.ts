import { beforeEach, describe, expect, it } from 'vitest';
import { useRecentWorkbenchesStore } from './index';

const wb = (id: string, name = `WB ${id}`) => ({
  workbenchId: id,
  workspaceId: 'ws-1',
  name,
});

describe('useRecentWorkbenchesStore', () => {
  beforeEach(() => {
    useRecentWorkbenchesStore.setState({ items: [] });
  });

  it('records newest first', () => {
    const { record } = useRecentWorkbenchesStore.getState();
    record(wb('a'));
    record(wb('b'));
    expect(useRecentWorkbenchesStore.getState().items.map((i) => i.workbenchId)).toEqual([
      'b',
      'a',
    ]);
  });

  it('dedupes by workbenchId, moving a revisit to the front with the fresh name', () => {
    const { record } = useRecentWorkbenchesStore.getState();
    record(wb('a', 'Old name'));
    record(wb('b'));
    record(wb('a', 'New name'));
    const items = useRecentWorkbenchesStore.getState().items;
    expect(items.map((i) => i.workbenchId)).toEqual(['a', 'b']);
    expect(items[0]?.name).toBe('New name');
  });

  it('caps at 5 items', () => {
    const { record } = useRecentWorkbenchesStore.getState();
    for (const id of ['a', 'b', 'c', 'd', 'e', 'f']) record(wb(id));
    const items = useRecentWorkbenchesStore.getState().items;
    expect(items).toHaveLength(5);
    expect(items[0]?.workbenchId).toBe('f');
    expect(items.some((i) => i.workbenchId === 'a')).toBe(false);
  });
});
