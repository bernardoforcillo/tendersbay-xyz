import { cn } from '@tendersbay/components/core';
import { ChevronDown, Quote } from 'lucide-react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { useState } from 'react';
import { Button, Disclosure, DisclosurePanel } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation, CitationSurface } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import { CitationRef } from '~/features/tenders/components/atoms';
import type { CitationView } from '~/features/tenders/constants';
import { useTenderPassages } from '~/features/tenders/hooks';

export type EvidenceQuoteProps = {
  /** Stringified tender id — the passage read is scoped to one tender. */
  tenderId: string;
  /** The verbatim span, as published. Rendered as-is: translating it would be translating data. */
  excerpt: string;
  citation: CitationView;
  /** Analytics surface for `citation_opened`. */
  location: AnalyticsLocation;
  /** Which surface the quote sits on — 'dossier' on the scheda gara, 'chat' in a reply. */
  surface?: CitationSurface;
  className?: string;
};

/**
 * A verbatim span from a tender document, its citation, and a one-click read of
 * the passage around it.
 *
 * This is the trust mechanism of the whole scheda gara: everything the product
 * concludes about a gara is checkable against the published text without leaving
 * the page. The quote is inline and always visible; the SURROUNDING passage is
 * behind a disclosure because retrieving it is a metered read, and firing one per
 * citation on mount would spend the capability on passages nobody opened.
 *
 * The expanded panel is `aria-live="polite"`: its content arrives asynchronously
 * after the press, so without a live region a screen-reader user gets silence
 * where a sighted user gets text.
 */
export function EvidenceQuote({
  tenderId,
  excerpt,
  citation,
  location,
  surface = 'dossier',
  className,
}: EvidenceQuoteProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const reduceMotion = useReducedMotion();

  // Component-local, and it stays local: the only trigger for this expansion is
  // the disclosure inside this component and the only reader is the panel below
  // it. Nothing outside needs to know which quote is open, so there is nothing
  // to promote to `store/scheda-gara`.
  const [expanded, setExpanded] = useState(false);
  const { data, loading, error, load } = useTenderPassages(tenderId);

  function handleExpandedChange(next: boolean) {
    setExpanded(next);
    if (!next) return;
    capture('citation_opened', {
      location,
      surface,
      // The only production read on whether Phase 1a's page/section metadata
      // actually landed. Never the page number itself, never the URL.
      has_section: (citation.sectionPath?.length ?? 0) > 0 || Boolean(citation.sectionTitle),
      has_page: Boolean(citation.pageStartSet),
    });
    // The excerpt is its own best query: we want the passage this span sits in.
    if (!data) void load(excerpt);
  }

  if (!excerpt) return <CitationRef citation={citation} className={className} />;

  return (
    <Disclosure
      isExpanded={expanded}
      onExpandedChange={handleExpandedChange}
      className={cn('flex flex-col gap-2', className)}
    >
      <blockquote className="flex gap-2 border-l-2 border-cream-300 pl-3">
        <Quote
          aria-hidden="true"
          strokeWidth={1.75}
          className="mt-0.5 h-3.5 w-3.5 shrink-0 text-ink-300"
        />
        <p className="text-sm italic text-ink-700">{excerpt}</p>
      </blockquote>

      <div className="flex flex-wrap items-center gap-3">
        <CitationRef citation={citation} />
        <Button
          slot="trigger"
          className={cn(
            'inline-flex min-h-10 items-center gap-1 rounded-lg px-2 text-xs font-semibold text-brand-700 outline-none',
            'transition-colors duration-150 data-[hovered]:bg-cream-100',
            'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600',
          )}
        >
          <ChevronDown
            aria-hidden="true"
            strokeWidth={1.75}
            className={cn(
              'h-3.5 w-3.5 transition-transform duration-150',
              expanded && 'rotate-180',
            )}
          />
          {expanded
            ? t('tenders.citation.collapsePassage', 'Hide the passage')
            : t('tenders.citation.expandPassage', 'Read the passage')}
        </Button>
      </div>

      <DisclosurePanel>
        <AnimatePresence initial={false}>
          {expanded && (
            <motion.div
              // Reduced motion renders the panel statically — the content is the
              // point, and an animated reveal is not information.
              initial={reduceMotion ? false : { opacity: 0, height: 0 }}
              animate={reduceMotion ? undefined : { opacity: 1, height: 'auto' }}
              exit={reduceMotion ? undefined : { opacity: 0, height: 0 }}
              transition={{ duration: 0.15 }}
              className="overflow-hidden"
            >
              <div
                aria-live="polite"
                className="flex flex-col gap-3 rounded-xl bg-cream-50 p-3 text-sm text-ink-700"
              >
                {/* Reuses the tenders namespace's existing retrieval string rather
                    than minting a 24-locale key for a spinner-less loading line. */}
                {loading && <p className="text-xs text-ink-500">{t('tenders.searching')}</p>}
                {error && <p className="text-xs text-ink-500">{error}</p>}
                {!loading &&
                  !error &&
                  data &&
                  (data.passages.length === 0 ? (
                    <p className="text-xs text-ink-500">
                      {t(
                        'tenders.citation.passageUnavailable',
                        'We hold no further text around this quote.',
                      )}
                    </p>
                  ) : (
                    data.passages.map((passage, i) => (
                      <div
                        // biome-ignore lint/suspicious/noArrayIndexKey: passages carry no id on the wire, and this list is fetched once in server order — it never reorders, so position is a stable identity here
                        key={`${passage.citation?.partIndex ?? 'p'}-${i}`}
                        className="flex flex-col gap-1"
                      >
                        <p className="whitespace-pre-wrap">{passage.text}</p>
                        {passage.truncated && (
                          <p className="text-xs text-ink-400">
                            {t('tenders.citation.truncated', 'Passage shortened.')}
                          </p>
                        )}
                        {passage.citation && <CitationRef citation={passage.citation} />}
                      </div>
                    ))
                  ))}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </DisclosurePanel>
    </Disclosure>
  );
}
