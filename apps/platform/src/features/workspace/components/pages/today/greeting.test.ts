import { describe, expect, it } from 'vitest';
import { greetingKey } from './greeting';

describe('greetingKey', () => {
  it.each([
    [0, 'morning'],
    [11, 'morning'],
    [12, 'afternoon'],
    [17, 'afternoon'],
    [18, 'evening'],
    [23, 'evening'],
  ] as const)('hour %i → %s', (hour, expected) => {
    expect(greetingKey(hour)).toBe(expected);
  });
});
