import { describe, expect, it } from 'vitest';
import { sidebarNavKeys } from './sidebar-nav';

describe('sidebarNavKeys', () => {
  it('shows only Explore outside a workspace', () => {
    expect(sidebarNavKeys(false)).toEqual(['explore']);
  });
  it('shows Overview, Explore and Settings inside a workspace', () => {
    expect(sidebarNavKeys(true)).toEqual(['overview', 'explore', 'settings']);
  });
});
