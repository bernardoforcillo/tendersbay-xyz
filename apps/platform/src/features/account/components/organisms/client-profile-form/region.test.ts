import { describe, expect, it } from 'vitest';
import { formatRegions, parseRegions } from './region';

describe('parseRegions', () => {
  it('splits comma-separated prefixes, trimming and uppercasing each', () => {
    expect(parseRegions('itc, de3')).toEqual(['ITC', 'DE3']);
  });

  it('drops empty segments from stray commas and whitespace', () => {
    expect(parseRegions(' itc,, de3 , ')).toEqual(['ITC', 'DE3']);
  });

  it('returns an empty array for blank input', () => {
    expect(parseRegions('')).toEqual([]);
    expect(parseRegions('   ')).toEqual([]);
  });
});

describe('formatRegions', () => {
  it('joins region codes with a comma-space separator', () => {
    expect(formatRegions(['ITC', 'DE3'])).toBe('ITC, DE3');
  });

  it('returns an empty string for an empty list', () => {
    expect(formatRegions([])).toBe('');
  });
});
