import { describe, expect, it } from 'vitest';
import { EU_COUNTRIES, FLAGS } from './flags';

describe('EU country flag data', () => {
  it('lists all 27 EU member states with no duplicates', () => {
    expect(EU_COUNTRIES).toHaveLength(27);
    expect(new Set(EU_COUNTRIES).size).toBe(27);
  });

  it('has a flag component for every country code', () => {
    for (const code of EU_COUNTRIES) {
      expect(FLAGS[code]).toBeTypeOf('function');
    }
  });
});
