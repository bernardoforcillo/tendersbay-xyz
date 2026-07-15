import type { HtmlTagDescriptor } from 'vite';

export interface HeadOptions {
  hostname: string;
  siteName: string;
  /** Page title for og:title/twitter:title; falls back to siteName when absent. */
  title?: string;
  description: string;
  ogImage?: string;
  twitterSite?: string;
  themeColor?: string;
  organization?: { name: string; url: string; logo?: string; sameAs?: string[] };
  /**
   * Optional schema.org Service node describing the product; added to the @graph
   * alongside Organization + WebSite. `provider` points back to the Organization.
   */
  service?: { name: string; description: string; serviceType?: string; areaServed?: string };
}

function absolutize(hostname: string, url: string): string {
  if (/^https?:\/\//.test(url)) {
    return url;
  }
  return `${hostname}${url.startsWith('/') ? '' : '/'}${url}`;
}

/**
 * Build the static <head> tags injected into index.html (identical across routes).
 *
 * Invariant: every value here comes from trusted, build-time plugin config (never
 * user input), so values are interpolated into HTML/JSON-LD without escaping. If a
 * dynamic source (e.g. a per-route title) is ever wired in, add HTML/JSON escaping.
 */
export function headTags(options: HeadOptions): HtmlTagDescriptor[] {
  const meta = (attrs: Record<string, string>): HtmlTagDescriptor => ({
    tag: 'meta',
    attrs,
    injectTo: 'head',
  });
  const image = options.ogImage ? absolutize(options.hostname, options.ogImage) : undefined;
  const title = options.title ?? options.siteName;
  const tags: HtmlTagDescriptor[] = [
    meta({ name: 'description', content: options.description }),
    meta({ property: 'og:type', content: 'website' }),
    meta({ property: 'og:site_name', content: options.siteName }),
    meta({ property: 'og:title', content: title }),
    meta({ property: 'og:description', content: options.description }),
    meta({ name: 'twitter:card', content: 'summary_large_image' }),
    meta({ name: 'twitter:title', content: title }),
    meta({ name: 'twitter:description', content: options.description }),
  ];
  if (image) {
    tags.push(meta({ property: 'og:image', content: image }));
    tags.push(meta({ name: 'twitter:image', content: image }));
  }
  if (options.twitterSite) {
    tags.push(meta({ name: 'twitter:site', content: options.twitterSite }));
  }
  if (options.themeColor) {
    tags.push(meta({ name: 'theme-color', content: options.themeColor }));
  }
  if (options.organization) {
    const org = options.organization;
    const graph: Record<string, unknown>[] = [
      {
        '@type': 'Organization',
        name: org.name,
        url: org.url,
        ...(org.logo ? { logo: org.logo } : {}),
        ...(org.sameAs?.length ? { sameAs: org.sameAs } : {}),
      },
      { '@type': 'WebSite', name: options.siteName, url: options.hostname },
    ];
    if (options.service) {
      const svc = options.service;
      graph.push({
        '@type': 'Service',
        name: svc.name,
        description: svc.description,
        ...(svc.serviceType ? { serviceType: svc.serviceType } : {}),
        ...(svc.areaServed ? { areaServed: svc.areaServed } : {}),
        provider: { '@type': 'Organization', name: org.name, url: org.url },
      });
    }
    const jsonLd = {
      '@context': 'https://schema.org',
      '@graph': graph,
    };
    tags.push({
      tag: 'script',
      attrs: { type: 'application/ld+json' },
      children: JSON.stringify(jsonLd),
      injectTo: 'head',
    });
  }
  return tags;
}
