import { describe, expect, it } from 'vitest';
import { buildLlmsTxt, DEFAULT_LLMS_TAGLINE } from './llms';

const opts = {
  hostname: 'https://tendersbay.xyz',
  siteName: 'tendersbay',
  locales: ['en-ie', 'it-it', 'de-de'] as const,
  description: 'Find EU tenders.',
};

describe('buildLlmsTxt', () => {
  it('leads with the site name and a value-prop blockquote', () => {
    const txt = buildLlmsTxt(opts);
    expect(txt).toContain('# tendersbay');
    expect(txt).toContain(`> ${DEFAULT_LLMS_TAGLINE}`);
  });

  it('defaults the intro paragraph to the product description', () => {
    expect(buildLlmsTxt(opts)).toContain('\nFind EU tenders.\n');
  });

  it('honours explicit tagline and intro overrides', () => {
    const txt = buildLlmsTxt({ ...opts, llmsTagline: 'Win more.', llmsIntro: 'What it is.' });
    expect(txt).toContain('> Win more.');
    expect(txt).toContain('\nWhat it is.\n');
  });

  it('lists a Key pages link per locale, pointing at the per-locale landing url', () => {
    const txt = buildLlmsTxt(opts);
    expect(txt).toContain('## Key pages');
    expect(txt).toContain('- [en-IE](https://tendersbay.xyz/en-ie/)');
    expect(txt).toContain('- [it-IT](https://tendersbay.xyz/it-it/)');
    expect(txt).toContain('- [de-DE](https://tendersbay.xyz/de-de/)');
    expect(txt.match(/^- \[/gm)).toHaveLength(3);
  });
});
