import { cn } from '@tendersbay/components/core';
import { ExternalLink } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { CitationView } from '~/features/tenders/constants';

export type CitationRefProps = {
  citation: CitationView;
  className?: string;
};

/**
 * Where a quoted passage came from, rendered as far down the ladder as the
 * metadata allows:
 *
 *   Disciplinare · § 7.2 Requisiti di capacità tecnica · p. 14
 *   Disciplinare · p. 14
 *   Disciplinare
 *
 * The ladder lives HERE, once. Every surface that quotes a document renders
 * through this component, so "citation metadata is optional by construction"
 * becomes true rather than merely intended — and a document extracted before the
 * extractor recorded pages (the normal case for anything already in the corpus,
 * not an edge case) degrades to a document reference instead of to "p. 0".
 *
 * Two things it must never emit: a bare "§" and a bare "p.". `pageStart` alone is
 * 0 for both page zero and "unknown", so `pageStartSet` — not truthiness — is
 * what gates the page fragment.
 */
export function CitationRef({ citation, className }: CitationRefProps) {
  const { t } = useTranslation();

  const fragments: string[] = [];

  // documentType is a published label ("Disciplinare", "Capitolato"). It is data,
  // not UI copy, so it is rendered verbatim; only its absence is localized.
  const label =
    citation.documentType?.trim() || t('tenders.citation.openDocument', 'Open document');
  fragments.push(label);

  const section = sectionLabel(citation);
  if (section) {
    fragments.push(t('tenders.citation.section', { section, defaultValue: '§ {{section}}' }));
  }

  if (citation.pageStartSet && citation.pageStart !== undefined) {
    // `start`/`end`, not `from`/`to`: these names ARE the contract with the 24
    // `tenders.citation.pageRange` strings, and i18next substitutes a variable
    // it was not given with '' rather than failing — a mismatch here ships
    // "pp. –" in every locale and nothing breaks loudly.
    const start = citation.pageStart;
    const end = citation.pageEnd;
    fragments.push(
      citation.pageEndSet && end !== undefined && end > start
        ? t('tenders.citation.pageRange', { start, end, defaultValue: 'pp. {{start}}–{{end}}' })
        : t('tenders.citation.page', { page: start, defaultValue: 'p. {{page}}' }),
    );
  }

  const text = fragments.join(' · ');
  const base = cn('inline-flex items-center gap-1 text-xs text-ink-500', className);

  if (!citation.documentUrl) return <span className={base}>{text}</span>;

  return (
    <a
      href={citation.documentUrl}
      target="_blank"
      rel="noopener noreferrer nofollow"
      className={cn(base, 'text-brand-700 underline underline-offset-2')}
    >
      {text}
      <ExternalLink aria-hidden="true" strokeWidth={1.75} className="h-3 w-3 shrink-0" />
    </a>
  );
}

/**
 * The deepest section label we can state. `sectionPath` is hierarchical
 * (["7", "7.2"]) and only its last element is worth showing next to a title —
 * "§ 7 7.2 Requisiti…" reads as a typo. Either half may be missing, and both
 * missing means no section fragment at all.
 */
function sectionLabel(citation: CitationView): string {
  const path = citation.sectionPath ?? [];
  const deepest = path.length > 0 ? (path[path.length - 1] ?? '').trim() : '';
  const title = (citation.sectionTitle ?? '').trim();
  return [deepest, title].filter(Boolean).join(' ');
}
