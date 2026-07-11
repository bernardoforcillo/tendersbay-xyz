import { describe, expect, it } from 'vitest';
import { cx } from './index';

describe('cx', () => {
  it('joins class strings with spaces', () => {
    expect(cx('a', 'b', 'c')).toBe('a b c');
  });

  it('skips falsy values', () => {
    expect(cx('a', false, undefined, null, 'b')).toBe('a b');
  });

  it('returns an empty string with no truthy args', () => {
    expect(cx(false, undefined)).toBe('');
  });
});
