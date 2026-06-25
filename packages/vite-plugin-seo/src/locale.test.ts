import { describe, expect, it } from 'vitest';
import { bcp47 } from './locale';

describe('bcp47', () => {
  it('uppercases the region subtag', () => {
    expect(bcp47('en-ie')).toBe('en-IE');
    expect(bcp47('de-de')).toBe('de-DE');
  });

  it('passes through a bare language subtag', () => {
    expect(bcp47('en')).toBe('en');
  });
});
