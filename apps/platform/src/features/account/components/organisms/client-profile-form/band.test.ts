import { describe, expect, it } from 'vitest';
import { band } from './band';

describe('band', () => {
  it('buckets 0', () => {
    expect(band(0)).toBe('0');
  });
  it('buckets 1', () => {
    expect(band(1)).toBe('1');
  });
  it('buckets 2-3', () => {
    expect(band(2)).toBe('2-3');
    expect(band(3)).toBe('2-3');
  });
  it('buckets 4+', () => {
    expect(band(4)).toBe('4+');
    expect(band(27)).toBe('4+');
  });
});
