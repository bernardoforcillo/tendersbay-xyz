import { describe, expect, it } from 'vitest';
import { seo } from './index';

const base = {
  hostname: 'https://tendersbay.xyz',
  locales: ['en-ie'] as const,
  defaultLocale: 'en-ie',
  siteName: 'tendersbay',
  description: 'Find EU tenders',
};

describe('seo', () => {
  it('returns a named plugin with a transformIndexHtml head handler', () => {
    const plugin = seo({ ...base });
    expect(plugin.name).toBe('tendersbay:seo');
    const hook = plugin.transformIndexHtml;
    const handler =
      typeof hook === 'function'
        ? hook
        : hook != null && 'handler' in hook
          ? hook.handler
          : undefined;
    const tags = handler?.('', { path: '/', filename: 'index.html' });
    expect(Array.isArray(tags)).toBe(true);
    expect(JSON.stringify(tags)).toContain('og:site_name');
  });
});
