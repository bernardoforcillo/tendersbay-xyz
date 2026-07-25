import { describe, expect, it } from 'vitest';
import { sidebarNavKeys } from './sidebar-nav';

describe('sidebarNavKeys', () => {
  it('returns only tenders without a workspace', () => {
    expect(sidebarNavKeys(false)).toEqual(['tenders']);
  });

  it('returns the four destinations with a workspace, in IA order', () => {
    expect(sidebarNavKeys(true)).toEqual(['today', 'tenders', 'explore', 'workbenches']);
  });
});
