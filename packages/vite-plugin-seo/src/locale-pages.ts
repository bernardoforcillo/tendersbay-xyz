import { bcp47 } from './locale.ts';

export interface LocaleMeta {
  title: string;
  description: string;
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
 * Rewrite the built index.html for one locale: localized <title>, description,
 * og/twitter title + description, `<html lang>` in BCP-47 casing, and an added
 * og:locale. Throws when any expected tag is absent (head-shape drift must not
 * silently ship unlocalized pages).
 *
 * Unlike the static head tags (see head.ts), these values flow in from the
 * locale copy files rather than plugin literals, so every interpolated value is
 * HTML-escaped — that is the escaping the head.ts invariant requires once
 * values stop being static config.
 */
export function localizeIndexHtml(html: string, locale: string, meta: LocaleMeta): string {
  const title = escapeHtml(meta.title);
  const description = escapeHtml(meta.description);

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
  return replaceOrThrow(
    out,
    /(<\/head)(>)/,
    (before, after) => `<meta property="og:locale" content="${ogLocale(locale)}">\n  ${before}${after}`,
    '</head>',
  );
}
