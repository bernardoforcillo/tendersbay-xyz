import { describe, expect, it } from 'vitest';
import { sanitizeRedirect } from './index';

describe('sanitizeRedirect', () => {
  it('keeps internal paths', () => {
    expect(sanitizeRedirect('/workspaces/abc')).toBe('/workspaces/abc');
    expect(sanitizeRedirect('/en-ie/workspace/accept-invite?token=x')).toBe(
      '/en-ie/workspace/accept-invite?token=x',
    );
  });

  it('rejects protocol-relative and absolute URLs (open-redirect guard)', () => {
    expect(sanitizeRedirect('//evil.com')).toBe('/');
    expect(sanitizeRedirect('https://evil.com')).toBe('/');
    expect(sanitizeRedirect('http://evil.com/path')).toBe('/');
  });

  it('defaults empty or nullish input to /', () => {
    expect(sanitizeRedirect(null)).toBe('/');
    expect(sanitizeRedirect(undefined)).toBe('/');
    expect(sanitizeRedirect('')).toBe('/');
  });
});
