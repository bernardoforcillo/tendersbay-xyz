import { describe, expect, it } from 'vitest';
import { normalizeOptions } from './options';

const base = {
  hostname: 'https://tendersbay.xyz/',
  locales: ['en-ie'] as const,
  defaultLocale: 'en-ie',
  siteName: 'tendersbay',
  description: 'Find EU tenders',
};

describe('normalizeOptions', () => {
  it('strips a trailing slash and defaults routesDir + lastmod', () => {
    const o = normalizeOptions({ ...base });
    expect(o.hostname).toBe('https://tendersbay.xyz');
    expect(o.routesDir).toBe('src/routes');
    expect(o.lastmod).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it('honours lastmod=false and an explicit lastmod string', () => {
    expect(normalizeOptions({ ...base, lastmod: false }).lastmod).toBeUndefined();
    expect(normalizeOptions({ ...base, lastmod: '2026-01-01' }).lastmod).toBe('2026-01-01');
  });
});
