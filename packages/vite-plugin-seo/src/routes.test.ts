import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { discoverRoutes } from './routes';

const FIX = fileURLToPath(new URL('../fixtures/routes', import.meta.url));

describe('discoverRoutes', () => {
  it('derives static locale-relative paths and skips layouts, dynamics, and the outer redirect', () => {
    expect(discoverRoutes(FIX)).toEqual(['/', '/about', '/pricing/']);
  });

  it('applies include and exclude', () => {
    expect(discoverRoutes(FIX, { include: ['/extra'], exclude: ['/about'] })).toEqual([
      '/',
      '/extra',
      '/pricing/',
    ]);
  });

  it('returns only include paths when the directory is missing', () => {
    expect(discoverRoutes(`${FIX}/does-not-exist`, { include: ['/only'] })).toEqual(['/only']);
  });
});
