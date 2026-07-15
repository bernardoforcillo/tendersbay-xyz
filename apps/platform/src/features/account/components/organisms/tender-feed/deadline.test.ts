import { describe, expect, it } from 'vitest';
import { deadlineInfo } from './deadline';

const ONE_DAY_MS = 86_400_000;
const now = new Date('2026-07-13T12:00:00.000Z');

function deadlineAt(daysFromNow: number): string {
  return new Date(now.getTime() + daysFromNow * ONE_DAY_MS).toISOString();
}

describe('deadlineInfo', () => {
  it('returns null for an empty deadline', () => {
    expect(deadlineInfo('', now)).toBeNull();
  });

  it('returns null for an unparseable deadline', () => {
    expect(deadlineInfo('not-a-date', now)).toBeNull();
  });

  it('tags an expired deadline as urgent with negative days', () => {
    expect(deadlineInfo(deadlineAt(-1), now)).toEqual({ tone: 'urgent', days: -1 });
  });

  it('tags a deadline due today as urgent with 0 days', () => {
    expect(deadlineInfo(deadlineAt(0), now)).toEqual({ tone: 'urgent', days: 0 });
  });

  it('tags exactly 7 days out as urgent', () => {
    expect(deadlineInfo(deadlineAt(7), now)).toEqual({ tone: 'urgent', days: 7 });
  });

  it('tags exactly 8 days out as deadline', () => {
    expect(deadlineInfo(deadlineAt(8), now)).toEqual({ tone: 'deadline', days: 8 });
  });

  it('tags exactly 14 days out as deadline', () => {
    expect(deadlineInfo(deadlineAt(14), now)).toEqual({ tone: 'deadline', days: 14 });
  });

  it('tags exactly 15 days out as neutral', () => {
    expect(deadlineInfo(deadlineAt(15), now)).toEqual({ tone: 'neutral', days: 15 });
  });
});
