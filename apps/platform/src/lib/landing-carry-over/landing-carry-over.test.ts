import { beforeEach, describe, expect, it } from 'vitest';
import {
  LANDING_CARRY_OVER_KEY,
  readAndClearLandingCarryOver,
  writeLandingCarryOver,
} from './index';

describe('landing carry-over', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('writes the query and filters under the stable key', () => {
    writeLandingCarryOver({ query: 'road works', filters: { country: 'IT' } });
    expect(JSON.parse(sessionStorage.getItem(LANDING_CARRY_OVER_KEY) ?? 'null')).toEqual({
      query: 'road works',
      filters: { country: 'IT' },
    });
  });

  it('reads and clears the carry-over — a second read returns null', () => {
    writeLandingCarryOver({ query: 'road works', filters: {} });
    expect(readAndClearLandingCarryOver()).toEqual({ query: 'road works', filters: {} });
    expect(sessionStorage.getItem(LANDING_CARRY_OVER_KEY)).toBeNull();
    expect(readAndClearLandingCarryOver()).toBeNull();
  });

  it('returns null when nothing was written', () => {
    expect(readAndClearLandingCarryOver()).toBeNull();
  });

  it('returns null and clears malformed JSON', () => {
    sessionStorage.setItem(LANDING_CARRY_OVER_KEY, 'not json');
    expect(readAndClearLandingCarryOver()).toBeNull();
  });

  it('returns null for a blank query', () => {
    sessionStorage.setItem(LANDING_CARRY_OVER_KEY, JSON.stringify({ query: '   ', filters: {} }));
    expect(readAndClearLandingCarryOver()).toBeNull();
  });

  it('defaults filters to an empty object when omitted', () => {
    sessionStorage.setItem(LANDING_CARRY_OVER_KEY, JSON.stringify({ query: 'roads' }));
    expect(readAndClearLandingCarryOver()).toEqual({ query: 'roads', filters: {} });
  });
});
