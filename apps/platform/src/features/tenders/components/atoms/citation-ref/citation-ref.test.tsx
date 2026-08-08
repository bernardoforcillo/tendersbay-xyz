import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import enIe from '~/assets/locales/en-ie/common.json';

// Interpolating stub over the REAL en-ie bundle.
//
// Two things this has to get right, and the second is why it reads the bundle
// rather than the call site's `defaultValue`:
//
//  1. It interpolates, so a `t` that returned raw "{{page}}" cannot let
//     "p. {{page}}" pass a "no bare p." assertion a real render would fail.
//  2. It resolves the key from the SHIPPED string first. A component that
//     passes `{ from, to }` to a locale string written "pp. {{start}}–{{end}}"
//     renders "pp. –" in production while passing every assertion here, because
//     i18next substitutes a variable it was not given with '' instead of
//     failing. Resolving `defaultValue` would test the component against its own
//     copy of the string — the one side of the contract that cannot disagree
//     with itself. `tenders-keys.test.ts` guards the other direction (the key
//     exists in all 24 locales and carries its placeholders); together they
//     close the gap neither closes alone.
function lookup(key: string): string | undefined {
  let node: unknown = enIe;
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) return undefined;
    node = (node as Record<string, unknown>)[part];
  }
  return typeof node === 'string' ? node : undefined;
}

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opt?: string | Record<string, unknown>) => {
      if (typeof opt === 'string') return opt;
      const template = lookup(key) ?? (opt?.defaultValue as string) ?? key;
      return template.replace(/{{(\w+)}}/g, (_m, name: string) => String(opt?.[name] ?? ''));
    },
    i18n: { language: 'en-ie' },
  }),
}));

import { CitationRef } from './index';

describe('CitationRef', () => {
  it('renders document, section and page when all three are known', () => {
    render(
      <CitationRef
        citation={{
          documentUrl: 'https://buyer.example/disciplinare.pdf',
          documentType: 'Disciplinare',
          sectionPath: ['7', '7.2'],
          sectionTitle: 'Requisiti di capacità tecnica',
          pageStart: 14,
          pageStartSet: true,
        }}
      />,
    );
    const link = screen.getByRole('link');
    expect(link.textContent).toContain('Disciplinare');
    expect(link.textContent).toContain('§ 7.2 Requisiti di capacità tecnica');
    expect(link.textContent).toContain('p. 14');
  });

  it('renders a page range when the passage spans pages', () => {
    render(
      <CitationRef
        citation={{
          documentUrl: 'https://buyer.example/d.pdf',
          documentType: 'Capitolato',
          pageStart: 14,
          pageStartSet: true,
          pageEnd: 16,
          pageEndSet: true,
        }}
      />,
    );
    expect(screen.getByRole('link').textContent).toContain('pp. 14–16');
  });

  // The middle rung: page but no section. This is the normal case for anything
  // extracted before the extractor recorded section headings.
  it('degrades to document and page when the section is unknown', () => {
    render(
      <CitationRef
        citation={{
          documentUrl: 'https://buyer.example/d.pdf',
          documentType: 'Disciplinare',
          pageStart: 3,
          pageStartSet: true,
          sectionPath: [],
          sectionTitle: '',
        }}
      />,
    );
    const text = screen.getByRole('link').textContent ?? '';
    expect(text).toContain('p. 3');
    expect(text).not.toContain('§');
  });

  it('degrades to the document alone when no page or section is known', () => {
    render(
      <CitationRef
        citation={{ documentUrl: 'https://buyer.example/d.pdf', documentType: 'Disciplinare' }}
      />,
    );
    const text = screen.getByRole('link').textContent ?? '';
    expect(text).toContain('Disciplinare');
    expect(text).not.toContain('§');
    expect(text).not.toMatch(/p\./);
  });

  // page_start is 0 for both "page zero" and "unknown"; only pageStartSet tells
  // them apart. Rendering "p. 0" here would undermine the whole affordance.
  it('never renders a page when pageStartSet is false, even at page 0', () => {
    render(
      <CitationRef
        citation={{
          documentUrl: 'https://buyer.example/d.pdf',
          documentType: 'Disciplinare',
          pageStart: 0,
          pageStartSet: false,
        }}
      />,
    );
    expect(screen.getByRole('link').textContent).not.toMatch(/p\./);
  });

  it('renders plain text with no link when the document URL is unknown', () => {
    render(<CitationRef citation={{ documentUrl: '', documentType: 'Disciplinare' }} />);
    expect(screen.queryByRole('link')).toBeNull();
    expect(screen.getByText('Disciplinare')).toBeTruthy();
  });

  it('falls back to a localized label when the document type is unpublished', () => {
    render(<CitationRef citation={{ documentUrl: 'https://buyer.example/d.pdf' }} />);
    expect(screen.getByRole('link').textContent).toContain('Open document');
  });
});
