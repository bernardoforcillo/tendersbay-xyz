import { describe, expect, it } from 'vitest';
import { buildSitemap } from './sitemap';

const opts = {
  hostname: 'https://tendersbay.xyz',
  locales: ['en-ie', 'de-de'] as const,
  defaultLocale: 'en-ie',
};

describe('buildSitemap', () => {
  it('emits one url block per locale with all hreflang alternates + x-default', () => {
    const xml = buildSitemap(['/'], opts);
    expect(xml).toContain('<loc>https://tendersbay.xyz/en-ie/</loc>');
    expect(xml).toContain('<loc>https://tendersbay.xyz/de-de/</loc>');
    expect(xml).toContain('hreflang="en-IE" href="https://tendersbay.xyz/en-ie/"');
    expect(xml).toContain('hreflang="de-DE" href="https://tendersbay.xyz/de-de/"');
    expect(xml).toContain('hreflang="x-default" href="https://tendersbay.xyz/en-ie/"');
    expect(xml.match(/<url>/g)).toHaveLength(2);
  });

  it('builds non-root paths with the locale prefix', () => {
    const xml = buildSitemap(['/about'], opts);
    expect(xml).toContain('<loc>https://tendersbay.xyz/en-ie/about</loc>');
  });

  it('includes lastmod / changefreq / priority when configured', () => {
    const xml = buildSitemap(['/'], {
      ...opts,
      lastmod: '2026-06-25',
      routeMeta: { '/': { changefreq: 'weekly', priority: 1 } },
    });
    expect(xml).toContain('<lastmod>2026-06-25</lastmod>');
    expect(xml).toContain('<changefreq>weekly</changefreq>');
    expect(xml).toContain('<priority>1</priority>');
  });
});
