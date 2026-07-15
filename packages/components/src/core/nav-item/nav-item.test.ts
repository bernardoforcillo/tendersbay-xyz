import { describe, expect, it } from 'vitest';
import { navItemClass, tabClass } from './index';

describe('navItemClass', () => {
  it('styles the current page via aria-current', () => {
    expect(navItemClass).toContain('[&[aria-current=page]]:bg-cream-200');
  });

  it('keeps the 40px minimum target height', () => {
    expect(navItemClass).toContain('py-2.5');
  });
});

describe('tabClass', () => {
  it('styles the current page via aria-current', () => {
    expect(tabClass).toContain('[&[aria-current=page]]:bg-cream-200');
  });
});
