import { describe, expect, it } from 'vitest';
import { escapeHtml, localizeIndexHtml, ogLocale } from './locale-pages';

const builtIndex = [
  '<!doctype html>',
  '<html lang="en">',
  '  <head>',
  '    <meta charset="UTF-8">',
  '    <title>tendersbay platform</title>',
  '    <meta name="description" content="default description">',
  '    <meta property="og:type" content="website">',
  '    <meta property="og:site_name" content="tendersbay">',
  '    <meta property="og:title" content="default title">',
  '    <meta property="og:description" content="default description">',
  '    <meta name="twitter:card" content="summary_large_image">',
  '    <meta name="twitter:title" content="default title">',
  '    <meta name="twitter:description" content="default description">',
  '  </head>',
  '  <body></body>',
  '</html>',
].join('\n');

const meta = { title: 'Bandi di gara — tendersbay', description: 'Gare pubbliche europee.' };

describe('escapeHtml', () => {
  it('escapes the five HTML-significant characters', () => {
    expect(escapeHtml('a & b <c> "d" \'e\'')).toBe('a &amp; b &lt;c&gt; &quot;d&quot; &#39;e&#39;');
  });
});

describe('ogLocale', () => {
  it('converts to the underscore og:locale form', () => {
    expect(ogLocale('it-it')).toBe('it_IT');
    expect(ogLocale('en-ie')).toBe('en_IE');
  });
});

describe('localizeIndexHtml', () => {
  const html = localizeIndexHtml(builtIndex, 'it-it', meta);

  it('replaces title, description, og and twitter tags with the locale values', () => {
    expect(html).toContain('<title>Bandi di gara — tendersbay</title>');
    expect(html).toContain('<meta name="description" content="Gare pubbliche europee.">');
    expect(html).toContain('<meta property="og:title" content="Bandi di gara — tendersbay">');
    expect(html).toContain('<meta property="og:description" content="Gare pubbliche europee.">');
    expect(html).toContain('<meta name="twitter:title" content="Bandi di gara — tendersbay">');
    expect(html).toContain('<meta name="twitter:description" content="Gare pubbliche europee.">');
    expect(html).not.toContain('default title');
    expect(html).not.toContain('default description');
  });

  it('sets <html lang> to the BCP-47 form and adds og:locale', () => {
    expect(html).toContain('<html lang="it-IT">');
    expect(html).toContain('<meta property="og:locale" content="it_IT">');
  });

  it('leaves untouched tags (charset, og:type, site_name, twitter:card) intact', () => {
    expect(html).toContain('<meta charset="UTF-8">');
    expect(html).toContain('<meta property="og:type" content="website">');
    expect(html).toContain('<meta property="og:site_name" content="tendersbay">');
    expect(html).toContain('<meta name="twitter:card" content="summary_large_image">');
  });

  it('HTML-escapes interpolated values so copy cannot break out of the head', () => {
    const hostile = localizeIndexHtml(builtIndex, 'fr-fr', {
      title: 'Appels <script>alert(1)</script> & "offres"',
      description: "l'attribution <b>",
    });
    expect(hostile).not.toContain('<script>alert(1)</script>');
    expect(hostile).toContain(
      '<title>Appels &lt;script&gt;alert(1)&lt;/script&gt; &amp; &quot;offres&quot;</title>',
    );
    expect(hostile).toContain('content="l&#39;attribution &lt;b&gt;"');
  });

  it('throws instead of silently no-opping when an expected tag is absent', () => {
    const missingTwitter = builtIndex.replace(
      '    <meta name="twitter:title" content="default title">\n',
      '',
    );
    expect(() => localizeIndexHtml(missingTwitter, 'it-it', meta)).toThrowError(
      /twitter:title.*not found/,
    );

    const missingLang = builtIndex.replace('<html lang="en">', '<html>');
    expect(() => localizeIndexHtml(missingLang, 'it-it', meta)).toThrowError(
      /<html lang>.*not found/,
    );

    const missingTitle = builtIndex.replace('    <title>tendersbay platform</title>\n', '');
    expect(() => localizeIndexHtml(missingTitle, 'it-it', meta)).toThrowError(
      /<title>.*not found/,
    );
  });
});
