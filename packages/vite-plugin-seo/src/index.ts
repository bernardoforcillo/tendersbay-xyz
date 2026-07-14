import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import type { Plugin } from 'vite';
import { headTags } from './head.ts';
import { localizeIndexHtml } from './locale-pages.ts';
import { normalizeOptions, type SeoOptions } from './options.ts';
import { buildRobots } from './robots.ts';
import { discoverRoutes } from './routes.ts';
import { buildSitemap } from './sitemap.ts';

export type { SeoOptions } from './options.ts';

/** Vite plugin: inject static SEO head tags and emit robots.txt + sitemap.xml. */
export function seo(options: SeoOptions): Plugin {
  const opts = normalizeOptions(options);
  let root = process.cwd();
  let outDir = 'dist';
  return {
    name: 'tendersbay:seo',
    configResolved(config) {
      root = config.root;
      outDir = config.build.outDir;
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
    // Runs after the bundle hits disk: re-emit index.html per locale with
    // localized head tags, so crawlers get locale-correct titles/descriptions
    // without JavaScript. The Go server routes /<locale>/ to these files.
    writeBundle(output) {
      const localeMeta = opts.localeMeta;
      if (!localeMeta) {
        return;
      }
      const dir = output.dir ?? path.resolve(root, outDir);
      const indexPath = path.join(dir, 'index.html');
      if (!existsSync(indexPath)) {
        this.warn(`localeMeta is set but ${indexPath} was not emitted; skipping locale pages`);
        return;
      }
      const html = readFileSync(indexPath, 'utf8');
      for (const [locale, meta] of Object.entries(localeMeta)) {
        let localized: string;
        try {
          localized = localizeIndexHtml(html, locale, meta);
        } catch (cause) {
          // A failed rewrite means head-shape drift: fail the build rather than
          // ship every locale page with default-locale copy.
          this.error(`locale page emission failed for ${locale}: ${(cause as Error).message}`);
        }
        mkdirSync(path.join(dir, locale), { recursive: true });
        writeFileSync(path.join(dir, locale, 'index.html'), localized);
      }
      // A leftover <dir>/index.html not covered by localeMeta is a stale locale
      // page (emptyOutDir may be off): it would be embedded and served as-is.
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        if (!entry.isDirectory() || localeMeta[entry.name]) {
          continue;
        }
        if (existsSync(path.join(dir, entry.name, 'index.html'))) {
          this.warn(
            `stale ${entry.name}/index.html in ${dir} is not in localeMeta; delete the directory or add the locale`,
          );
        }
      }
    },
  };
}
