import path from 'node:path';
import type { Plugin } from 'vite';
import { headTags } from './head.ts';
import { normalizeOptions, type SeoOptions } from './options.ts';
import { buildRobots } from './robots.ts';
import { discoverRoutes } from './routes.ts';
import { buildSitemap } from './sitemap.ts';

export type { SeoOptions } from './options.ts';

/** Vite plugin: inject static SEO head tags and emit robots.txt + sitemap.xml. */
export function seo(options: SeoOptions): Plugin {
  const opts = normalizeOptions(options);
  let root = process.cwd();
  return {
    name: 'tendersbay:seo',
    configResolved(config) {
      root = config.root;
    },
    transformIndexHtml: {
      order: 'pre',
      handler() {
        return headTags(opts);
      },
    },
    generateBundle() {
      const routesDir = path.resolve(root, opts.routesDir);
      const paths = discoverRoutes(routesDir, { include: opts.include, exclude: opts.exclude });
      this.emitFile({ type: 'asset', fileName: 'robots.txt', source: buildRobots(opts) });
      this.emitFile({ type: 'asset', fileName: 'sitemap.xml', source: buildSitemap(paths, opts) });
    },
  };
}
