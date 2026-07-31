import { cn } from '@tendersbay/components/core';
import type { CpvMatch } from '@tendersbay/proto/tender/v1/tender_pb';
import { X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Shows the CPV codes the server read out of the query text.
 *
 * This is what makes cross-language search explicable. Typing "pulizie uffici"
 * surfaces German and French notices because the phrase resolved to CPV 90919200,
 * and without this the user has no way to tell that from the engine returning
 * unrelated results — nor any way to correct it.
 *
 * Unlike AppliedFilterChips, each chip here is individually removable: these are
 * separate inferences, and a query that resolves to three codes can easily be
 * right about two of them. Removal round-trips to the server rather than just
 * hiding the chip: the lexicon resolves the same code from the same text on
 * every request, so a client-side-only removal would reappear on the next page
 * or the next load. `onRemove` is expected to re-run the search with the code
 * added to `suppressedCpv`.
 *
 * The label is NOT translated here — it is the official CPV label the server
 * matched, already in a real language, and re-rendering it through i18n would
 * mean shipping 9.450 codes of app copy to say what the vocabulary already says.
 *
 * Deliberately renders with NO leading "Read from your query:" label of its
 * own — nor does its sibling AppliedFilterChips. The two are stacked
 * together on the tenders page, and each family can be empty independently
 * (a query can resolve a CPV code with zero applied filters, or vice versa),
 * so neither component can safely own the label itself without either
 * showing it twice (both present) or not at all (only one present, if each
 * only showed it conditionally on ITS OWN content). The tenders page renders
 * the shared label once, gated on either family having something to show,
 * above both. Each chip here already spells out "CPV {code} — {label}",
 * which carries its own context regardless.
 */
export function AppliedCpvChips({
  matches,
  onRemove,
}: {
  /** Codes the server inferred, strongest first. */
  matches: CpvMatch[];
  /** Called with the code to stop inferring. */
  onRemove: (code: string) => void;
}) {
  const { t } = useTranslation();
  if (matches.length === 0) return null;

  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      {matches.map((m) => {
        const chip = `${t('tenders.applied.cpv', { code: m.code })} — ${m.label}`;
        return (
          <span
            key={m.code}
            className={cn(
              'inline-flex items-center gap-1 rounded-full border border-brand-200 bg-brand-50',
              'px-2.5 py-0.5 text-xs font-medium text-brand-800',
            )}
          >
            {chip}
            <button
              type="button"
              aria-label={t('tenders.applied.remove', { chip })}
              onClick={() => onRemove(m.code)}
              className={cn(
                'rounded-full outline-none',
                'hover:text-brand-950 focus-visible:ring-2 focus-visible:ring-brand-600',
              )}
            >
              <X size={12} aria-hidden="true" />
            </button>
          </span>
        );
      })}
    </div>
  );
}
