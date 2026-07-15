import { bcp47 } from './locale.ts';
import type { FaqItem, LocaleHero } from './options.ts';

export interface LocaleMeta {
  title: string;
  description: string;
}

/** Everything needed to re-emit the built index.html for one locale. */
export interface LocalePageContext {
  hostname: string;
  locales: readonly string[];
  defaultLocale: string;
  meta: LocaleMeta;
  /** When present, a FAQPage JSON-LD block + a <noscript> Q&A list are emitted. */
  faq?: FaqItem[];
  /** When present, a <noscript> hero + FAQ content block is emitted. */
  hero?: LocaleHero;
}

/** Escape a value for interpolation into HTML text or a double-quoted attribute. */
export function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

/**
 * Serialize a value as JSON safe to embed inside a `<script>` element: JSON.stringify
 * escapes quotes/backslashes, then `<`, `>`, `&` become `\u00XX` so a `</script>` or
 * comment sequence in the locale copy cannot break out of the element.
 */
export function jsonForScript(value: unknown): string {
  return JSON.stringify(value)
    .replaceAll('<', '\\u003c')
    .replaceAll('>', '\\u003e')
    .replaceAll('&', '\\u0026');
}

/** og:locale underscore form (`it-it` -> `it_IT`). */
export function ogLocale(locale: string): string {
  return bcp47(locale).replace('-', '_');
}

/**
 * Replace one pattern match, or throw. The rewrite targets are conventionally
 * coupled to the head shape Vite emits; a silent no-op here would ship all
 * locale pages with default-locale copy, so a missing tag must fail the build.
 */
function replaceOrThrow(
  html: string,
  pattern: RegExp,
  build: (before: string, after: string) => string,
  what: string,
): string {
  let matched = false;
  const out = html.replace(pattern, (_match, before: string, after: string) => {
    matched = true;
    return build(before, after);
  });
  if (!matched) {
    throw new Error(`locale head rewrite: ${what} not found in the built index.html`);
  }
  return out;
}

/** Replace the content attribute of the meta tag identified by `attr="value"`. */
function setMetaContent(html: string, attr: string, value: string, content: string): string {
  const pattern = new RegExp(`(<meta\\s+${attr}="${value}"\\s+content=")[^"]*(")`);
  return replaceOrThrow(
    html,
    pattern,
    (before, after) => `${before}${content}${after}`,
    `<meta ${attr}="${value}">`,
  );
}

/**
 * schema.org FAQPage JSON-LD `<script>` from a locale's Q&A. This is the single
 * highest-value GEO/AEO signal — answer engines quote FAQ structured data
 * verbatim. Values are locale copy, so they flow through `jsonForScript`.
 */
export function faqPageScript(faq: FaqItem[]): string {
  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: faq.map((item) => ({
      '@type': 'Question',
      name: item.question,
      acceptedAnswer: { '@type': 'Answer', text: item.answer },
    })),
  };
  return `<script type="application/ld+json">${jsonForScript(jsonLd)}</script>`;
}

/**
 * Per-locale canonical (self), the full hreflang alternate set across every
 * locale plus x-default -> default locale, and an og:locale:alternate per other
 * locale. Values derive from trusted locale codes + config hostname, so no
 * escaping is needed. A distinct page per locale exists, so a self-canonical is
 * correct (this reverses the plugin's earlier no-canonical decision).
 */
export function alternateLinks(
  hostname: string,
  locale: string,
  locales: readonly string[],
  defaultLocale: string,
): string {
  const url = (loc: string) => `${hostname}/${loc}/`;
  const lines: string[] = [`<link rel="canonical" href="${url(locale)}">`];
  for (const loc of locales) {
    lines.push(`<link rel="alternate" hreflang="${bcp47(loc)}" href="${url(loc)}">`);
  }
  lines.push(`<link rel="alternate" hreflang="x-default" href="${url(defaultLocale)}">`);
  for (const loc of locales) {
    if (loc !== locale) {
      lines.push(`<meta property="og:locale:alternate" content="${ogLocale(loc)}">`);
    }
  }
  return lines.join('\n  ');
}

/**
 * A `<noscript>` block carrying the locale's real hero + FAQ copy. This is the
 * actual page content shown when JS is off (the SPA otherwise renders an empty
 * shell) and to AI crawlers — honest, not cloaking. All copy is HTML-escaped.
 */
export function noscriptBlock(hero: LocaleHero, faq?: FaqItem[]): string {
  const parts: string[] = [
    '<noscript>',
    `<h1>${escapeHtml(hero.headline)}</h1>`,
    `<p>${escapeHtml(hero.subtitle)}</p>`,
  ];
  if (faq?.length) {
    parts.push('<dl>');
    for (const item of faq) {
      parts.push(`<dt>${escapeHtml(item.question)}</dt>`, `<dd>${escapeHtml(item.answer)}</dd>`);
    }
    parts.push('</dl>');
  }
  parts.push('</noscript>');
  return parts.join('\n    ');
}

/**
 * Rewrite the built index.html for one locale: localized <title>, description,
 * og/twitter title + description, `<html lang>` in BCP-47 casing, og:locale, a
 * self-canonical + full hreflang set + og:locale:alternate, an optional FAQPage
 * JSON-LD block, and an optional <noscript> content block. Throws when any
 * expected tag is absent (head-shape drift must not silently ship unlocalized
 * pages).
 *
 * Unlike the static head tags (see head.ts), title/description/faq/hero flow in
 * from the locale copy files rather than plugin literals, so every interpolated
 * copy value is HTML- or JSON-escaped — that is the escaping the head.ts
 * invariant requires once values stop being static config.
 */
export function localizeIndexHtml(html: string, locale: string, ctx: LocalePageContext): string {
  const title = escapeHtml(ctx.meta.title);
  const description = escapeHtml(ctx.meta.description);

  let out = replaceOrThrow(
    html,
    /(<html[^>]*\slang=")[^"]*(")/,
    (before, after) => `${before}${bcp47(locale)}${after}`,
    '<html lang>',
  );
  out = replaceOrThrow(
    out,
    /(<title>)[^<]*(<\/title>)/,
    (before, after) => `${before}${title}${after}`,
    '<title>',
  );
  out = setMetaContent(out, 'name', 'description', description);
  out = setMetaContent(out, 'property', 'og:title', title);
  out = setMetaContent(out, 'property', 'og:description', description);
  out = setMetaContent(out, 'name', 'twitter:title', title);
  out = setMetaContent(out, 'name', 'twitter:description', description);

  const headAdditions: string[] = [
    `<meta property="og:locale" content="${ogLocale(locale)}">`,
    alternateLinks(ctx.hostname, locale, ctx.locales, ctx.defaultLocale),
  ];
  if (ctx.faq?.length) {
    headAdditions.push(faqPageScript(ctx.faq));
  }
  out = replaceOrThrow(
    out,
    /(<\/head)(>)/,
    (before, after) => `${headAdditions.join('\n  ')}\n  ${before}${after}`,
    '</head>',
  );

  if (ctx.hero) {
    const block = noscriptBlock(ctx.hero, ctx.faq);
    out = replaceOrThrow(
      out,
      /(<\/body)(>)/,
      (before, after) => `  ${block}\n  ${before}${after}`,
      '</body>',
    );
  }
  return out;
}
