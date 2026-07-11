import { describe, expect, it } from 'vitest';
import { sidebarNavKeys } from './sidebar-nav';

describe('sidebarNavKeys', () => {
  it('returns only explore without a workspace', () => {
    expect(sidebarNavKeys(false)).toEqual(['explore']);
  });

  it('returns the three destinations with a workspace, in IA order', () => {
    expect(sidebarNavKeys(true)).toEqual(['today', 'explore', 'workbenches']);
  });
});
