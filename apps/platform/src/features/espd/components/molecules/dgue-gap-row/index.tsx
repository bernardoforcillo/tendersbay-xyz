import { cn } from '@tendersbay/components/core';
import type { Gap } from '@tendersbay/proto/espd/v1/espd_pb';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import {
  type GapReason,
  type GapScope,
  isNationalGround,
  parseNationalGround,
} from '~/features/espd/constants';

export type DgueGapRowProps = {
  gap: Gap;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /**
   * Where the reader goes to close it. The row does not capture inline: a Part
   * III answer is a legal declaration and a Part II.A field is a registry fact,
   * and neither belongs in a one-line control next to eleven others.
   */
  onOpen: (gap: Gap) => void;
  className?: string;
};

/**
 * One open field of the DGUE, said in a sentence a person can act on.
 *
 * The sentence is built from three things and never from a status colour: WHAT
 * is missing (the criterion and, when the criterion has several, the field), WHY
 * it is open, and WHERE it is fixed. "Not authoritative" is the one worth
 * spelling out — the fact exists, the agent or an import put it there, and this
 * document only carries what a human stated. A red dot would say none of that.
 *
 * There is no severity here and no ordering by it. Every gap on this list blocks
 * the export equally, and ranking them would invent a priority the document does
 * not have.
 */
export function DgueGapRow({ gap, location, onOpen, className }: DgueGapRowProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();

  const reason = gap.reason as GapReason;
  const scope = gap.scope as GapScope;

  function open() {
    capture('dgue_gap_filled', { location, scope, reason });
    onOpen(gap);
  }

  return (
    <li
      className={cn(
        'flex flex-col gap-1 border-cream-200 border-b py-2 last:border-b-0',
        className,
      )}
    >
      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
        <span className="font-medium text-ink-900 text-sm">{criterionLabel(gap.criterion, t)}</span>
        {gap.field && gap.field !== 'answer' && (
          <span className="text-ink-500 text-xs">{fieldLabel(gap.field, t)}</span>
        )}
      </div>
      <p className="text-ink-500 text-xs">
        {t(`espd.gap.reason.${reason}`, REASON_FALLBACK[reason] ?? reason)}
      </p>
      <button
        type="button"
        onClick={open}
        className="self-start text-brand-700 text-xs underline-offset-2 hover:underline"
      >
        {t(
          scope === 'company' ? 'espd.gap.openDossier' : 'espd.gap.openBid',
          scope === 'company' ? 'Open the dossier' : 'Fill it in for this tender',
        )}
      </button>
    </li>
  );
}

/**
 * Names a criterion, falling back to the raw key rather than to nothing.
 *
 * Part III.D is the open half of the vocabulary: national law defines those
 * grounds, so there is no closed list to translate and the label is built from
 * the country and the national code the operator's own lawyer used.
 */
export function criterionLabel(criterion: string, t: TranslateFn): string {
  if (isNationalGround(criterion)) {
    const { country, code } = parseNationalGround(criterion);
    return `${t(`espd.criteria.iii.d.purely_national_grounds`, 'Purely national exclusion grounds')} — ${country} ${code}`;
  }
  return t(`espd.criteria.${criterion}`, criterion);
}

/**
 * The slice of `t` the helpers below need. Typed as i18next's own `TFunction`
 * rather than as a hand-written `(key, fallback) => string`: the real signature
 * is overloaded, and re-declaring a narrower one makes every call site cast.
 */
export type TranslateFn = TFunction<'translation', undefined>;

/**
 * Names the field inside a criterion.
 *
 * The identity scalars borrow the dossier form's own labels rather than getting
 * a second set: they are the same fields, already translated into 24 languages,
 * and a second copy would drift into two words for one input.
 */
export function fieldLabel(field: string, t: TranslateFn): string {
  const shared = SHARED_FIELD_KEYS[field];
  if (shared) return t(shared, field);
  return t(`espd.fields.${field}`, FIELD_FALLBACK[field] ?? field);
}

const SHARED_FIELD_KEYS: Record<string, string> = {
  legal_name: 'company.dossier.identity.legalName',
  vat_number: 'company.dossier.identity.vatNumber',
  fiscal_code: 'company.dossier.identity.fiscalCode',
  legal_form: 'company.dossier.identity.legalForm',
  country: 'company.dossier.identity.country',
  nuts: 'company.dossier.identity.nuts',
};

const FIELD_FALLBACK: Record<string, string> = {
  is_sme: 'SME status',
  representative: 'Legal representative',
  buyer_name: 'Contracting authority',
  reference: 'Procedure reference',
  lot_ref: 'Lot',
  confirmation: 'Confirmation for this tender',
  criterion: 'Requested by the buyer',
};

const REASON_FALLBACK: Record<GapReason, string> = {
  missing: 'Nobody has answered this yet.',
  not_authoritative:
    'We hold a value, but nobody stated it — this document only carries facts a person confirmed.',
  stale: 'Answered, but not re-confirmed for this tender since it last changed.',
};
