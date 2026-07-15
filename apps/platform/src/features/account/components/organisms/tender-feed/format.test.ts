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

  it('formats with it-it grouping and no decimals', () => {
    const result = formatTenderValue(1_234_567n, 'EUR', 'it-it');
    expect(result).not.toBeNull();
    expect(result).toContain('€');
    expect(result).toMatch(/1\.234\.567/);
    expect(result).not.toMatch(/[,.]\d{2}\s*€?$/);
  });

  it('formats with en-ie grouping, symbol leading, and no decimals', () => {
    const result = formatTenderValue(1_234_567n, 'EUR', 'en-ie');
    expect(result).not.toBeNull();
    expect(result?.startsWith('€')).toBe(true);
    expect(result).toMatch(/1,234,567/);
  });
});
