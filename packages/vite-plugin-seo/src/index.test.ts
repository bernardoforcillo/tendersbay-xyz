import { existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { describe, expect, it, vi } from 'vitest';
import { seo } from './index';

const base = {
  hostname: 'https://tendersbay.xyz',
  locales: ['en-ie'] as const,
  defaultLocale: 'en-ie',
  siteName: 'tendersbay',
  description: 'Find EU tenders',
};

/** Minimal PluginContext stub: warn records, error throws (rollup's contract). */
function pluginContext() {
  return {
    warn: vi.fn(),
    error: (message: unknown) => {
      throw new Error(String(message));
    },
  };
}

function runWriteBundle(plugin: ReturnType<typeof seo>, ctx: unknown, dir: string) {
  const hook = plugin.writeBundle;
  const handler = typeof hook === 'function' ? hook : hook?.handler;
  handler?.call(ctx as never, { dir } as never, {} as never);
}

describe('seo', () => {
  it('returns a named plugin with a transformIndexHtml head handler', () => {
    const plugin = seo({ ...base });
    expect(plugin.name).toBe('tendersbay:seo');
    const hook = plugin.transformIndexHtml;
    const handler =
      typeof hook === 'function'
        ? hook
        : hook != null && 'handler' in hook
          ? hook.handler
          : undefined;
    const tags = handler?.('', { path: '/', filename: 'index.html' });
    expect(Array.isArray(tags)).toBe(true);
    expect(JSON.stringify(tags)).toContain('og:site_name');
  });

  it('writeBundle emits a localized <locale>/index.html per localeMeta entry', () => {
    const plugin = seo({
      ...base,
      locales: ['en-ie', 'it-it'] as const,
      title: 'EU public tenders — tendersbay',
      localeMeta: {
        'it-it': { title: 'Bandi di gara — tendersbay', description: 'Gare pubbliche europee.' },
      },
      localeFaq: {
        'it-it': [{ question: 'I miei dati addestrano la AI?', answer: 'No. Restano tuoi.' }],
      },
      localeHero: {
        'it-it': { headline: 'La gara data per loro? Aggiudicata.', subtitle: 'Finisce qui.' },
      },
    });
    const dir = mkdtempSync(path.join(tmpdir(), 'seo-plugin-'));
    writeFileSync(
      path.join(dir, 'index.html'),
      '<!doctype html><html lang="en"><head><title>app</title>' +
        '<meta name="description" content="d"><meta property="og:title" content="t">' +
        '<meta property="og:description" content="d"><meta name="twitter:title" content="t">' +
        '<meta name="twitter:description" content="d"></head><body></body></html>',
    );

    runWriteBundle(plugin, pluginContext(), dir);

    const emitted = readFileSync(path.join(dir, 'it-it', 'index.html'), 'utf8');
    expect(emitted).toContain('<title>Bandi di gara — tendersbay</title>');
    expect(emitted).toContain('<meta name="description" content="Gare pubbliche europee.">');
    expect(emitted).toContain('<html lang="it-IT">');
    expect(emitted).toContain('<meta property="og:locale" content="it_IT">');
    // Per-locale self-canonical + hreflang + FAQPage + noscript content block.
    expect(emitted).toContain('<link rel="canonical" href="https://tendersbay.xyz/it-it/">');
    expect(emitted).toContain('hreflang="x-default" href="https://tendersbay.xyz/en-ie/"');
    expect(emitted).toContain('"@type":"FAQPage"');
    expect(emitted).toContain('<noscript>');
    expect(emitted).toContain('<h1>La gara data per loro? Aggiudicata.</h1>');
  });

  it('generateBundle emits robots.txt, sitemap.xml, and llms.txt', () => {
    const plugin = seo({ ...base, locales: ['en-ie', 'it-it'] as const });
    const emitted: Record<string, string> = {};
    const ctx = {
      emitFile: (asset: { fileName: string; source: string }) => {
        emitted[asset.fileName] = asset.source;
      },
      warn: vi.fn(),
    };
    const hook = plugin.generateBundle;
    const handler = typeof hook === 'function' ? hook : hook?.handler;
    handler?.call(ctx as never, {} as never, {} as never, {} as never);

    expect(Object.keys(emitted).sort()).toEqual(['llms.txt', 'robots.txt', 'sitemap.xml']);
    expect(emitted['robots.txt']).toContain('User-agent: GPTBot');
    expect(emitted['robots.txt']).toContain('https://tendersbay.xyz/llms.txt');
    expect(emitted['llms.txt']).toContain('# tendersbay');
    expect(emitted['llms.txt']).toContain('## Key pages');
    expect(emitted['llms.txt']).toContain('- [it-IT](https://tendersbay.xyz/it-it/)');
  });

  it('writeBundle warns and emits nothing when index.html is missing', () => {
    const plugin = seo({
      ...base,
      localeMeta: { 'it-it': { title: 't', description: 'd' } },
    });
    const dir = mkdtempSync(path.join(tmpdir(), 'seo-plugin-'));
    const ctx = pluginContext();

    runWriteBundle(plugin, ctx, dir);

    expect(ctx.warn).toHaveBeenCalledOnce();
    expect(String(ctx.warn.mock.calls[0]?.[0])).toContain('was not emitted');
    expect(existsSync(path.join(dir, 'it-it', 'index.html'))).toBe(false);
  });

  it('writeBundle fails the build when the head shape drifted', () => {
    const plugin = seo({
      ...base,
      localeMeta: { 'it-it': { title: 't', description: 'd' } },
    });
    const dir = mkdtempSync(path.join(tmpdir(), 'seo-plugin-'));
    // No og/twitter tags at all: every meta rewrite must miss.
    writeFileSync(
      path.join(dir, 'index.html'),
      '<!doctype html><html lang="en"><head><title>app</title></head><body></body></html>',
    );

    expect(() => runWriteBundle(plugin, pluginContext(), dir)).toThrowError(
      /locale page emission failed for it-it/,
    );
  });

  it('writeBundle warns about a stale locale dir not covered by localeMeta', () => {
    const plugin = seo({
      ...base,
      localeMeta: { 'it-it': { title: 't', description: 'd' } },
    });
    const dir = mkdtempSync(path.join(tmpdir(), 'seo-plugin-'));
    writeFileSync(
      path.join(dir, 'index.html'),
      '<!doctype html><html lang="en"><head><title>app</title>' +
        '<meta name="description" content="d"><meta property="og:title" content="t">' +
        '<meta property="og:description" content="d"><meta name="twitter:title" content="t">' +
        '<meta name="twitter:description" content="d"></head><body></body></html>',
    );
    // A leftover page from a removed locale.
    const stale = path.join(dir, 'xx-xx');
    mkdirSync(stale, { recursive: true });
    writeFileSync(path.join(stale, 'index.html'), '<!doctype html>');
    const ctx = pluginContext();

    runWriteBundle(plugin, ctx, dir);

    expect(ctx.warn).toHaveBeenCalledWith(expect.stringContaining('stale xx-xx/index.html'));
  });
});
