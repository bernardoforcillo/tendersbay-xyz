import { bcp47 } from './locale.ts';

export interface LlmsOptions {
  hostname: string;
  siteName: string;
  locales: readonly string[];
  /** Product description; the default source for the intro paragraph. */
  description: string;
  /** One-line value prop for the `>` blockquote. */
  llmsTagline?: string;
  /** "What tendersbay is" paragraph. Defaults to `description`. */
  llmsIntro?: string;
}

/** Default value prop when `llmsTagline` is not supplied. */
export const DEFAULT_LLMS_TAGLINE =
  'A team of AI agents that find, prepare and help SMEs win public tenders across the EU — in all 24 official EU languages.';

/**
 * Build `llms.txt` — the emerging "robots.txt for LLMs": a clean Markdown brief
 * that answer engines and AI crawlers can read to understand the site. No
 * invented metrics; every value flows from build-time config.
 *
 * Invariant: siteName, hostname, tagline, intro, and locale codes come from
 * trusted build-time plugin config (never user input), so they are interpolated
 * into Markdown without escaping.
 */
export function buildLlmsTxt(options: LlmsOptions): string {
  const tagline = options.llmsTagline ?? DEFAULT_LLMS_TAGLINE;
  const intro = options.llmsIntro ?? options.description;
  const pages = options.locales.map(
    (locale) => `- [${bcp47(locale)}](${options.hostname}/${locale}/)`,
  );
  return [
    `# ${options.siteName}`,
    '',
    `> ${tagline}`,
    '',
    intro,
    '',
    '## Key pages',
    '',
    ...pages,
    '',
  ].join('\n');
}
