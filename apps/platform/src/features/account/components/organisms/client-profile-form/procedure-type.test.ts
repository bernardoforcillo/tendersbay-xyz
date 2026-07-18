import { describe, expect, it } from 'vitest';
import { PROCEDURE_TYPES } from './procedure-type';

describe('PROCEDURE_TYPES', () => {
  it('matches the backend clientprofile.go closed set exactly', () => {
    expect(PROCEDURE_TYPES).toEqual([
      'open',
      'restricted',
      'negotiated',
      'competitive_dialogue',
      'innovation_partnership',
      'other',
    ]);
  });

  it('has no duplicate values', () => {
    expect(new Set(PROCEDURE_TYPES).size).toBe(PROCEDURE_TYPES.length);
  });
});
