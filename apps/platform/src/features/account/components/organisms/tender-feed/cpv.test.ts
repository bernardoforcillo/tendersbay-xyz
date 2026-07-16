import { describe, expect, it } from 'vitest';
import { CPV_SECTORS, cpvPrefix } from './cpv';

describe('cpv sectors', () => {
  it('maps every sector key to its CPV prefix', () => {
    for (const { key, prefix } of CPV_SECTORS) {
      expect(cpvPrefix(key)).toBe(prefix);
    }
  });

  it('returns undefined for an unknown or empty key', () => {
    expect(cpvPrefix('nope')).toBeUndefined();
    expect(cpvPrefix('')).toBeUndefined();
  });

  it('uses unique keys and two-digit numeric prefixes', () => {
    const keys = CPV_SECTORS.map((s) => s.key);
    expect(new Set(keys).size).toBe(keys.length);
    for (const { prefix } of CPV_SECTORS) {
      expect(prefix).toMatch(/^\d{2}$/);
    }
  });
});
