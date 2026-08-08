import { cn, Pill } from '@tendersbay/components/core';
import { Check } from 'lucide-react';
import { useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type {
  AnalyticsLocation,
  ReportedRequirementKind,
  ReportedRequirementSource,
} from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import type { RequirementView } from '~/features/company/constants';
import { EvidenceQuote } from '~/features/tenders';
import { citationFromRequirement } from '~/features/tenders/constants';
import { companyClient } from '~/lib/api/client';

export type RequirementRowProps = {
  workspaceId: string;
  requirement: RequirementView;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Refetch requirements (and the assessment) after a confirm or a removal. */
  onChanged: () => void;
  className?: string;
};

/**
 * One captured participation requirement, with the passage it was read from and
 * the human lever over it.
 *
 * The lever is confirm/remove and deliberately not "add" or "edit the text".
 * A `user_stated` requirement counts as authoritative, so a free-text create
 * button would let a user manufacture an authoritative requirement out of
 * nothing and then be told by the product that they do not qualify for it. The
 * correct human act over a machine extraction is to check it against the quoted
 * passage and either vouch for it or delete it.
 *
 * An unconfirmed extraction may raise a QUESTION but never a verdict, which is
 * why the confirm affordance is prominent and carries its own explanation rather
 * than sitting silently in a corner: confirming is what upgrades a guess into
 * something the go/no-go is allowed to rest on.
 */
export function RequirementRow({
  workspaceId,
  requirement,
  location,
  onChanged,
  className,
}: RequirementRowProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [busy, setBusy] = useState(false);

  const confirmed = Boolean(requirement.confirmedBy);
  // `user_stated` needs no confirmation — the user IS the source.
  const needsConfirmation = !confirmed && requirement.source !== 'user_stated';
  const citation = citationFromRequirement(requirement);

  async function confirm() {
    setBusy(true);
    try {
      await companyClient.confirmTenderRequirement({
        workspaceId,
        requirementId: requirement.id,
      });
      capture('requirement_confirmed', {
        location,
        kind: (requirement.kind ?? '') as ReportedRequirementKind,
        source: (requirement.source ?? '') as ReportedRequirementSource,
        // Booleans only — never the excerpt, never the document URL.
        had_excerpt: Boolean(requirement.excerpt),
        had_citation: Boolean(requirement.documentUrl),
      });
      onChanged();
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    setBusy(true);
    try {
      await companyClient.removeTenderRequirement({
        workspaceId,
        requirementId: requirement.id,
      });
      onChanged();
    } finally {
      setBusy(false);
    }
  }

  return (
    <li className={cn('flex flex-col gap-2 py-4', className)}>
      <div className="flex flex-wrap items-center gap-2">
        <Pill tone="neutral">
          {t(
            `company.requirements.kind.${requirement.kind}`,
            KIND_FALLBACK[requirement.kind] ?? requirement.kind,
          )}
        </Pill>
        <Pill tone="neutral" className={cn(!confirmed && 'grayscale')}>
          {t(
            `company.requirements.source.${requirement.source}`,
            SOURCE_FALLBACK[requirement.source] ?? requirement.source,
          )}
        </Pill>
        <span className="text-xs text-ink-400">
          {requirement.lotRef
            ? t('company.requirements.lotLabel', {
                lot: requirement.lotRef,
                defaultValue: 'Lot {{lot}}',
              })
            : t('company.requirements.noticeLevel', 'Applies to every lot')}
        </span>
      </div>

      {/* As published — this is what a human checks the parse against, so it is
          never paraphrased and never translated. */}
      <p className="text-sm text-ink-900">{requirement.requirementText}</p>

      {requirement.excerpt ? (
        <EvidenceQuote
          tenderId={requirement.tenderId ?? ''}
          excerpt={requirement.excerpt}
          citation={citation}
          location={location}
        />
      ) : (
        requirement.documentUrl && <EvidenceQuoteFallback documentUrl={requirement.documentUrl} />
      )}

      <div className="flex flex-wrap items-center gap-2">
        {confirmed ? (
          <span className="inline-flex items-center gap-1 text-xs font-semibold text-brand-700">
            <Check aria-hidden="true" strokeWidth={1.75} className="h-3.5 w-3.5" />
            {t('company.requirements.confirmed', 'You confirmed this')}
          </span>
        ) : (
          needsConfirmation && (
            <>
              <Button isDisabled={busy} onPress={() => void confirm()} className={ACTION_BUTTON}>
                {t('company.requirements.confirm', 'Confirm against the passage')}
              </Button>
              <p className="text-xs text-ink-400">
                {t(
                  'company.requirements.unconfirmedHint',
                  'Unconfirmed extractions raise questions, never verdicts.',
                )}
              </p>
            </>
          )
        )}
        <Button isDisabled={busy} onPress={() => void remove()} className={QUIET_BUTTON}>
          {t('company.requirements.remove', 'Remove')}
        </Button>
      </div>
    </li>
  );
}

/** A requirement with a document but no quoted span still names its source. */
function EvidenceQuoteFallback({ documentUrl }: { documentUrl: string }) {
  const { t } = useTranslation();
  return (
    <a
      href={documentUrl}
      target="_blank"
      rel="noopener noreferrer nofollow"
      className="w-fit text-xs text-brand-700 underline underline-offset-2"
    >
      {t('tenders.citation.openDocument', 'Open document')}
    </a>
  );
}

const ACTION_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl border border-cream-300 bg-white px-4 text-sm font-semibold text-ink-800 outline-none ' +
  'transition-colors duration-150 data-[hovered]:border-cream-400 data-[pressed]:bg-cream-100 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

const QUIET_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl px-3 text-xs font-semibold text-ink-500 outline-none ' +
  'transition-colors duration-150 data-[hovered]:bg-cream-200 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

/**
 * English fallbacks for the requirement kinds. `company.requirements.kind.*` is
 * outside the Phase 4 key inventory, so until it lands these carry the label —
 * a bare "past_contracts" in a pill is a wire constant leaking into the product.
 */
const KIND_FALLBACK: Record<string, string> = {
  soa: 'SOA',
  certification: 'Certification',
  turnover: 'Turnover',
  past_contracts: 'Past contracts',
  headcount: 'Headcount',
  registration: 'Registration',
  other: 'Other',
};

const SOURCE_FALLBACK: Record<string, string> = {
  user_stated: 'You stated this',
  document_excerpt: 'Read from a document',
  award_criteria: 'From the award criteria',
  agent_inferred: 'Inferred by the agent',
};
