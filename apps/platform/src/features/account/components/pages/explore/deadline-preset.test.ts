import { describe, expect, it } from 'vitest';
import { DEADLINE_PRESETS, deadlineRange } from './deadline-preset';

describe('deadlineRange', () => {
  const now = new Date('2026-07-16T00:00:00.000Z');

  it('returns null for "any time" (no preset)', () => {
    expect(deadlineRange(null, now)).toBeNull();
  });

  it('builds an RFC3339 window from now to now + preset days', () => {
    expect(deadlineRange(7, now)).toEqual({
      from: '2026-07-16T00:00:00.000Z',
      to: '2026-07-23T00:00:00.000Z',
    });
    expect(deadlineRange(30, now)?.to).toBe('2026-08-15T00:00:00.000Z');
    expect(deadlineRange(90, now)?.to).toBe('2026-10-14T00:00:00.000Z');
  });

  it('exposes the presets in ascending order', () => {
    expect([...DEADLINE_PRESETS]).toEqual([7, 30, 90]);
  });
});
