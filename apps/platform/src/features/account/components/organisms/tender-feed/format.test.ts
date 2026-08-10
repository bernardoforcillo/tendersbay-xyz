import { describe, expect, it } from 'vitest';
import { formatTenderValue } from './format';

describe('formatTenderValue', () => {
  it('returns null when value is zero', () => {
    expect(formatTenderValue(0n, 'EUR', 'en-ie')).toBeNull();
  });

  it('returns null when value is negative', () => {
    expect(formatTenderValue(-1n, 'EUR', 'en-ie')).toBeNull();
  });

  it('returns null when currency is empty', () => {
    expect(formatTenderValue(1_000n, '', 'en-ie')).toBeNull();
  });

  it('returns null when the currency code is invalid instead of throwing', () => {
    expect(formatTenderValue(1_000n, 'NOTACURRENCY', 'en-ie')).toBeNull();
  });

  it('reads the value as minor units, not whole euros', () => {
    // The regression: 629_724_005 cents is €6,297,240.05, and used to render
    // as €629,724,005 — a hundredfold overstatement on every tender on screen.
    expect(formatTenderValue(629_724_005n, 'EUR', 'en-ie')).toBe('€6,297,240.05');
  });

  it('omits cents when the notice carries none', () => {
    expect(formatTenderValue(540_400_000n, 'EUR', 'en-ie')).toBe('€5,404,000');
  });

  it('formats with it-it grouping and a trailing symbol', () => {
    const result = formatTenderValue(123_456_700n, 'EUR', 'it-it');
    expect(result).not.toBeNull();
    expect(result).toContain('€');
    expect(result).toMatch(/1\.234\.567/);
  });

  it('formats with en-ie grouping and a leading symbol', () => {
    const result = formatTenderValue(123_456_700n, 'EUR', 'en-ie');
    expect(result).not.toBeNull();
    expect(result?.startsWith('€')).toBe(true);
    expect(result).toMatch(/1,234,567/);
  });
});
