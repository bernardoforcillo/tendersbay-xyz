export interface FaqItem {
  question: string;
  answer: string;
}

export interface LocaleHero {
  headline: string;
  subtitle: string;
}

export interface ServiceNode {
  name: string;
  description: string;
  serviceType?: string;
  areaServed?: string;
}

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
  /**
   * Per-locale FAQ Q&A. When present for a locale, a schema.org FAQPage JSON-LD
   * block is emitted into that locale's dist/<locale>/index.html — the highest-
   * value GEO/AEO signal, quoted verbatim by answer engines.
   */
  localeFaq?: Record<string, FaqItem[]>;
  /**
   * Per-locale hero copy. When present for a locale, a <noscript> content block
   * (real hero + FAQ copy) is injected into that locale's page so non-JS clients
   * and AI crawlers receive the actual page content, not the empty SPA shell.
   */
  localeHero?: Record<string, LocaleHero>;
  ogImage?: string;
  twitterSite?: string;
  themeColor?: string;
  organization?: { name: string; url: string; logo?: string; sameAs?: string[] };
  /**
   * Optional schema.org Service node added to the head JSON-LD @graph, describing
   * the product itself (provider -> Organization, areaServed, serviceType).
   */
  service?: ServiceNode;
  /**
   * AI crawler user-agents explicitly allowed in robots.txt (each gets its own
   * `Allow: /` block). Defaults to the major answer-engine/LLM crawlers.
   */
  aiCrawlers?: string[];
  /** One-line value prop for the `>` blockquote in llms.txt. */
  llmsTagline?: string;
  /** "What tendersbay is" paragraph in llms.txt. Defaults to `description`. */
  llmsIntro?: string;
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
