import { cn } from '@tendersbay/components/core';
import { useTranslation } from 'react-i18next';
import type { GapView } from '~/features/company/constants';

export type RemedyLineProps = {
  gap: GapView;
  className?: string;
};

/**
 * What would close this gap.
 *
 * A blocking gap with no remedy line reads as unsolvable, and for an Italian SOA
 * or turnover shortfall that is usually false — avvalimento and RTI exist exactly
 * for it. So the remedy KIND is localized and shown next to the published
 * requirement text and what the dossier holds, which together are enough to act
 * on without the server having to compose a sentence.
 *
 * The remedy's structured details (which classifica is missing, by how much the
 * turnover falls short) stay off the wire for now: the kind plus the requirement
 * plus `held` already tell the reader what to do, and shipping the arithmetic
 * would invite the UI to re-derive a domain decision. Revisit only if measurement
 * shows this line being ignored.
 */
export function RemedyLine({ gap, className }: RemedyLineProps) {
  const { t } = useTranslation();
  const remedy = gap.remedy ?? '';
  if (!remedy) return null;

  return (
    <li className={cn('text-sm text-ink-700', className)}>
      <span className="font-medium">
        {t(`company.eligibility.remedy.${remedy}`, REMEDY_FALLBACK[remedy] ?? remedy)}
      </span>
      {gap.requirement?.requirementText && (
        // As-published prose, in the gara's own language — quoted data, never
        // translated.
        <span className="text-ink-500"> — {gap.requirement.requirementText}</span>
      )}
    </li>
  );
}

const REMEDY_FALLBACK: Record<string, string> = {
  provide_fact: 'Tell us what you hold',
  renew: 'Renew before the deadline',
  upgrade: 'Upgrade the attestation',
  avvalimento: 'Borrow the requirement (avvalimento)',
  rti: 'Bid as a temporary grouping (RTI)',
  subcontract: 'Cover it by subcontracting',
};
