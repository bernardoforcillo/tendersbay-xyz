import { bcp47 } from './locale.ts';

export interface SitemapOptions {
  hostname: string;
  locales: readonly string[];
  defaultLocale: string;
  lastmod?: string;
  routeMeta?: Record<string, { changefreq?: string; priority?: number }>;
}

/** Absolute URL for a locale + locale-relative path. `path` always starts with `/`. */
function urlFor(hostname: string, locale: string, path: string): string {
  return `${hostname}/${locale}${path}`;
}

function alternates(options: SitemapOptions, path: string): string {
  const { hostname, locales, defaultLocale } = options;
  const links = locales.map(
    (loc) =>
      `    <xhtml:link rel="alternate" hreflang="${bcp47(loc)}" href="${urlFor(hostname, loc, path)}"/>`,
  );
  links.push(
    `    <xhtml:link rel="alternate" hreflang="x-default" href="${urlFor(hostname, defaultLocale, path)}"/>`,
  );
  return links.join('\n');
}

function urlBlock(options: SitemapOptions, path: string, locale: string, links: string): string {
  const lines = ['  <url>', `    <loc>${urlFor(options.hostname, locale, path)}</loc>`, links];
  if (options.lastmod) {
    lines.push(`    <lastmod>${options.lastmod}</lastmod>`);
  }
  const meta = options.routeMeta?.[path];
  if (meta?.changefreq) {
    lines.push(`    <changefreq>${meta.changefreq}</changefreq>`);
  }
  if (meta?.priority !== undefined) {
    lines.push(`    <priority>${meta.priority}</priority>`);
  }
  lines.push('  </url>');
  return lines.join('\n');
}

/** Build a sitemap.xml expanding each path across every locale with hreflang alternates. */
export function buildSitemap(paths: string[], options: SitemapOptions): string {
  const blocks = paths.flatMap((path) => {
    const links = alternates(options, path);
    return options.locales.map((locale) => urlBlock(options, path, locale, links));
  });
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">',
    ...blocks,
    '</urlset>',
    '',
  ].join('\n');
}
