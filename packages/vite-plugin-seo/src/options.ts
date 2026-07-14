export interface SeoOptions {
  hostname: string;
  locales: readonly string[];
  defaultLocale: string;
  routesDir?: string;
  include?: string[];
  exclude?: string[];
  routeMeta?: Record<string, { changefreq?: string; priority?: number }>;
  lastmod?: boolean | string;
  siteName: string;
  /** Default-locale page title for og:title/twitter:title; falls back to siteName. */
  title?: string;
  description: string;
  /**
   * Per-locale <title>/description overrides. When set, the plugin re-emits the
   * built index.html as dist/<locale>/index.html with localized head tags.
   */
  localeMeta?: Record<string, { title: string; description: string }>;
  ogImage?: string;
  twitterSite?: string;
  themeColor?: string;
  organization?: { name: string; url: string; logo?: string; sameAs?: string[] };
}

export type NormalizedOptions = Omit<SeoOptions, 'lastmod' | 'routesDir'> & {
  routesDir: string;
  lastmod?: string;
};

function resolveLastmod(lastmod: SeoOptions['lastmod']): string | undefined {
  if (lastmod === false) {
    return undefined;
  }
  if (typeof lastmod === 'string') {
    return lastmod;
  }
  return new Date().toISOString().slice(0, 10);
}

/** Apply defaults and normalize raw plugin options into a clean shape. */
export function normalizeOptions(options: SeoOptions): NormalizedOptions {
  return {
    ...options,
    hostname: options.hostname.replace(/\/+$/, ''),
    routesDir: options.routesDir ?? 'src/routes',
    lastmod: resolveLastmod(options.lastmod),
  };
}
