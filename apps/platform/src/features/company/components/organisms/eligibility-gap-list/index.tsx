import { Card, cn } from '@tendersbay/components/core';
import { useRef } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation, ReportedCoverage, ReportedCoverageReason } from '~/analytics';
import { useCaptureEventOnce } from '~/analytics';
import { GapStatusPill } from '~/features/company/components/atoms';
import { CaptureQuestion, RemedyLine } from '~/features/company/components/molecules';
import type { AssessmentView, GapView } from '~/features/company/constants';
import { EvidenceQuote } from '~/features/tenders';
import { citationFromRequirement } from '~/features/tenders/constants';
import { useChatStore } from '~/store/chat';
import { useSchedaGaraStore } from '~/store/scheda-gara';

export type EligibilityGapListProps = {
  workspaceId: string;
  /** Stringified tender id. */
  tenderId: string;
  assessment: AssessmentView;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Re-run the eligibility check after a captured answer. */
  onCaptured: () => void;
  className?: string;
};

/**
 * Every requirement checked against the dossier, IN THE ORDER RECEIVED.
 *
 * The list is never re-sorted or re-grouped client-side. Which gap leads is a
 * domain decision the backend already made (blocking first, then by severity),
 * and re-deciding it here would put the same judgement in two places that will
 * eventually disagree — a boundary violation in spirit even where no import rule
 * catches it.
 *
 * An `unknown` gap renders its capture question already expanded: unknown means
 * nobody was ever asked, and this is the moment the answer is worth something.
 */
export function EligibilityGapList({
  workspaceId,
  tenderId,
  assessment,
  location,
  onCaptured,
  className,
}: EligibilityGapListProps) {
  const { t } = useTranslation();
  const gaps = assessment.gaps ?? [];

  // Once per ASSESSMENT, not per mount: a refetch triggered by an answered
  // question hands this component a new object, and a second event would inflate
  // the denominator of the gap-shown rate.
  useCaptureEventOnce(
    'eligibility_gap_shown',
    assessment.evaluatedAt ?? `${gaps.length}:${assessment.verdict}`,
    {
      location,
      gap_count: gaps.length,
      blocking: (assessment.blockingGapCount ?? 0) > 0,
      unknown_count: assessment.unknownGapCount ?? 0,
      // Coverage travels with the gap event so the two are joinable: a no-go
      // computed over the notice alone and one computed over the disciplinare
      // are not the same claim.
      coverage: (assessment.documentCoverage ?? '') as ReportedCoverage,
      reason: (assessment.documentReason ?? '') as ReportedCoverageReason,
    },
  );

  if (gaps.length === 0) {
    return (
      <Card className={cn('bg-cream-50', className)}>
        <p className="text-sm text-ink-500">
          {t(
            'company.requirements.empty',
            'No participation requirement has been captured for this gara yet.',
          )}
        </p>
      </Card>
    );
  }

  return (
    <ul className={cn('flex flex-col', className)}>
      {gaps.map((gap, i) => (
        <GapRow
          key={gap.requirement?.id ?? `${gap.status}-${i}`}
          workspaceId={workspaceId}
          tenderId={tenderId}
          gap={gap}
          location={location}
          onCaptured={onCaptured}
        />
      ))}
    </ul>
  );
}

function GapRow({
  workspaceId,
  tenderId,
  gap,
  location,
  onCaptured,
}: {
  workspaceId: string;
  tenderId: string;
  gap: GapView;
  location: AnalyticsLocation;
  onCaptured: () => void;
}) {
  const { t } = useTranslation();
  const setDraft = useChatStore((s) => s.setDraft);
  const setRepairOpen = useSchedaGaraStore((s) => s.setRepairOpen);
  const pillRef = useRef<HTMLSpanElement>(null);

  const requirement = gap.requirement;
  const citation = requirement ? citationFromRequirement(requirement) : null;

  function handleCaptured() {
    onCaptured();
    // The pill is the thing that changed. Without moving focus to it the answer
    // lands silently for a screen-reader user, and the whole payoff of answering
    // in place — seeing the verdict move — is sighted-only.
    pillRef.current?.focus();
  }

  function askWhy() {
    if (!requirement) return;
    // Reuses the ⌘K palette's one-shot draft handoff rather than adding a second
    // mechanism for the same job.
    setDraft(
      t('bid.scheda.repairChat.seedGapQuestion', {
        requirement: requirement.requirementText,
        held: gap.held ?? '',
        defaultValue: 'Why does “{{requirement}}” block this bid, and what can I do about it?',
      }),
    );
    setRepairOpen(true);
  }

  return (
    // gap-6 between rows (weakly related), margin owned by the row — the larger
    // element — so the list's rhythm does not depend on its container.
    <li className="flex flex-col gap-2 border-b border-cream-200 pb-6 last:border-b-0 last:pb-0 [&+li]:pt-6">
      <div className="flex flex-wrap items-center gap-2">
        <GapStatusPill ref={pillRef} status={gap.status} blocking={gap.blocking} />
        {requirement && (
          <Button onPress={askWhy} className={ASK_WHY_BUTTON}>
            {t('company.requirements.askWhy', 'Why?')}
          </Button>
        )}
      </div>

      {/* Requirement text and its evidence are strongly related — gap-2 keeps
          them reading as one claim. */}
      {requirement && <p className="text-sm text-ink-900">{requirement.requirementText}</p>}

      {gap.held && (
        <p className="text-sm text-ink-500">
          {t('company.eligibility.heldLabel', 'Your dossier holds')}:{' '}
          <span className="text-ink-700">{gap.held}</span>
        </p>
      )}

      {requirement?.excerpt && citation && (
        <EvidenceQuote
          tenderId={tenderId}
          excerpt={requirement.excerpt}
          citation={citation}
          location={location}
        />
      )}

      {gap.remedy && (
        <ul className="flex flex-col gap-1">
          <RemedyLine gap={gap} />
        </ul>
      )}

      {gap.status === 'unknown' && requirement && (
        <CaptureQuestion
          workspaceId={workspaceId}
          tenderId={tenderId}
          requirement={requirement}
          location={location}
          onSaved={handleCaptured}
        />
      )}
    </li>
  );
}

const ASK_WHY_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl px-3 text-xs font-semibold text-brand-700 outline-none ' +
  'transition-colors duration-150 data-[hovered]:bg-cream-100 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';
