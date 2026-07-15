import { describe, expect, it } from 'vitest';
import {
  alternateLinks,
  escapeHtml,
  faqPageScript,
  jsonForScript,
  type LocalePageContext,
  localizeIndexHtml,
  noscriptBlock,
  ogLocale,
} from './locale-pages';

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
const faq = [
  { question: 'I miei dati addestrano la vostra AI?', answer: 'No. Restano tuoi.' },
  { question: 'Si inventa le cose?', answer: 'Legge la gara e cita la pagina.' },
];
const hero = {
  headline: 'La gara che davano già per loro? Aggiudicata.',
  subtitle: 'Finisce qui.',
};

const ctx = (over: Partial<LocalePageContext> = {}): LocalePageContext => ({
  hostname: 'https://tendersbay.xyz',
  locales: ['en-ie', 'it-it', 'de-de'],
  defaultLocale: 'en-ie',
  meta,
  ...over,
});

describe('escapeHtml', () => {
  it('escapes the five HTML-significant characters', () => {
    expect(escapeHtml('a & b <c> "d" \'e\'')).toBe('a &amp; b &lt;c&gt; &quot;d&quot; &#39;e&#39;');
  });
});

describe('jsonForScript', () => {
  it('escapes <, >, & so copy cannot break out of a <script>', () => {
    const out = jsonForScript({ a: '</script><b> & "x"' });
    expect(out).not.toContain('</script>');
    expect(out).toContain('\\u003c/script\\u003e\\u003cb\\u003e \\u0026 ');
    // Still valid JSON once unescaped by the JSON parser inside a script context.
    expect(JSON.parse(out.replaceAll('\\u003c', '<').replaceAll('\\u003e', '>')).a).toBe(
      '</script><b> & "x"',
    );
  });
});

describe('ogLocale', () => {
  it('converts to the underscore og:locale form', () => {
    expect(ogLocale('it-it')).toBe('it_IT');
    expect(ogLocale('en-ie')).toBe('en_IE');
  });
});

describe('faqPageScript', () => {
  it('emits a FAQPage graph with one Question/Answer per item', () => {
    const script = faqPageScript(faq);
    expect(script).toContain('"@type":"FAQPage"');
    expect(script).toContain('"@type":"Question"');
    expect(script).toContain('"name":"I miei dati addestrano la vostra AI?"');
    expect(script).toContain('"acceptedAnswer":{"@type":"Answer","text":"No. Restano tuoi."}');
  });

  it('JSON-escapes hostile Q&A so it cannot break out of the script element', () => {
    const script = faqPageScript([{ question: '</script><img>', answer: 'x & y' }]);
    expect(script).not.toContain('</script><img>');
    expect(script).toContain('\\u003c/script\\u003e');
  });
});

describe('alternateLinks', () => {
  const links = alternateLinks(
    'https://tendersbay.xyz',
    'it-it',
    ['en-ie', 'it-it', 'de-de'],
    'en-ie',
  );

  it('emits a self-canonical for the locale', () => {
    expect(links).toContain('<link rel="canonical" href="https://tendersbay.xyz/it-it/">');
  });

  it('emits an hreflang alternate for every locale plus x-default -> default', () => {
    expect(links).toContain('hreflang="en-IE" href="https://tendersbay.xyz/en-ie/"');
    expect(links).toContain('hreflang="it-IT" href="https://tendersbay.xyz/it-it/"');
    expect(links).toContain('hreflang="de-DE" href="https://tendersbay.xyz/de-de/"');
    expect(links).toContain('hreflang="x-default" href="https://tendersbay.xyz/en-ie/"');
  });

  it('emits og:locale:alternate for the other locales, not the current one', () => {
    expect(links).toContain('<meta property="og:locale:alternate" content="en_IE">');
    expect(links).toContain('<meta property="og:locale:alternate" content="de_DE">');
    expect(links).not.toContain('<meta property="og:locale:alternate" content="it_IT">');
  });
});

describe('noscriptBlock', () => {
  it('renders the hero as h1/p and the FAQ as a dl, all HTML-escaped', () => {
    const block = noscriptBlock({ headline: 'Head <b>', subtitle: 'Sub & tail' }, [
      { question: 'Q <1>?', answer: 'A & 1' },
    ]);
    expect(block).toContain('<noscript>');
    expect(block).toContain('<h1>Head &lt;b&gt;</h1>');
    expect(block).toContain('<p>Sub &amp; tail</p>');
    expect(block).toContain('<dt>Q &lt;1&gt;?</dt>');
    expect(block).toContain('<dd>A &amp; 1</dd>');
    expect(block).not.toContain('<b>');
  });

  it('omits the dl when there is no FAQ', () => {
    const block = noscriptBlock({ headline: 'H', subtitle: 'S' });
    expect(block).not.toContain('<dl>');
  });
});

describe('localizeIndexHtml', () => {
  const html = localizeIndexHtml(builtIndex, 'it-it', ctx({ faq, hero }));

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

  it('injects a self-canonical, full hreflang set, and FAQPage JSON-LD in the head', () => {
    expect(html).toContain('<link rel="canonical" href="https://tendersbay.xyz/it-it/">');
    expect(html).toContain('hreflang="x-default" href="https://tendersbay.xyz/en-ie/"');
    expect(html).toContain('<meta property="og:locale:alternate" content="de_DE">');
    expect(html).toContain('"@type":"FAQPage"');
    // Head additions land before </head>, the noscript block before </body>.
    expect(html.indexOf('FAQPage')).toBeLessThan(html.indexOf('</head>'));
  });

  it('injects a <noscript> hero + FAQ content block before </body>', () => {
    expect(html).toContain('<noscript>');
    expect(html).toContain('<h1>La gara che davano già per loro? Aggiudicata.</h1>');
    expect(html).toContain('<dt>I miei dati addestrano la vostra AI?</dt>');
    expect(html.indexOf('<noscript>')).toBeLessThan(html.indexOf('</body>'));
    expect(html.indexOf('</head>')).toBeLessThan(html.indexOf('<noscript>'));
  });

  it('omits FAQPage and noscript when faq/hero are absent', () => {
    const bare = localizeIndexHtml(builtIndex, 'it-it', ctx());
    expect(bare).not.toContain('FAQPage');
    expect(bare).not.toContain('<noscript>');
    // canonical + hreflang are always emitted.
    expect(bare).toContain('<link rel="canonical" href="https://tendersbay.xyz/it-it/">');
  });

  it('leaves untouched tags (charset, og:type, site_name, twitter:card) intact', () => {
    expect(html).toContain('<meta charset="UTF-8">');
    expect(html).toContain('<meta property="og:type" content="website">');
    expect(html).toContain('<meta property="og:site_name" content="tendersbay">');
    expect(html).toContain('<meta name="twitter:card" content="summary_large_image">');
  });

  it('HTML-escapes interpolated values so copy cannot break out of the head', () => {
    const hostile = localizeIndexHtml(builtIndex, 'fr-fr', {
      hostname: 'https://tendersbay.xyz',
      locales: ['en-ie', 'fr-fr'],
      defaultLocale: 'en-ie',
      meta: {
        title: 'Appels <script>alert(1)</script> & "offres"',
        description: "l'attribution <b>",
      },
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
    expect(() => localizeIndexHtml(missingTwitter, 'it-it', ctx())).toThrowError(
      /twitter:title.*not found/,
    );

    const missingLang = builtIndex.replace('<html lang="en">', '<html>');
    expect(() => localizeIndexHtml(missingLang, 'it-it', ctx())).toThrowError(
      /<html lang>.*not found/,
    );

    const missingTitle = builtIndex.replace('    <title>tendersbay platform</title>\n', '');
    expect(() => localizeIndexHtml(missingTitle, 'it-it', ctx())).toThrowError(
      /<title>.*not found/,
    );
  });

  it('throws when a noscript block is requested but </body> is absent', () => {
    const noBody = builtIndex.replace('  <body></body>\n', '');
    expect(() => localizeIndexHtml(noBody, 'it-it', ctx({ hero }))).toThrowError(
      /<\/body>.*not found/,
    );
  });
});
