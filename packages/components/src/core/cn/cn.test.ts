import { describe, expect, it } from 'vitest';
import { cn } from './index';

describe('cn', () => {
  it('joins class strings with spaces', () => {
    expect(cn('a', 'b', 'c')).toBe('a b c');
  });

  it('skips falsy values', () => {
    expect(cn('a', false, undefined, null, 'b')).toBe('a b');
  });

  it('resolves conflicting Tailwind utilities, last one wins', () => {
    expect(cn('p-2 text-sm', 'p-4')).toBe('text-sm p-4');
  });

  it('returns an empty string with no truthy args', () => {
    expect(cn(false, undefined)).toBe('');
  });
});
