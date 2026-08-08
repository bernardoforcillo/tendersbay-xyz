import { cn } from '@tendersbay/components/core';
import { AlertTriangle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation, ReportedVerdict } from '~/analytics';
import { useCaptureEventOnce } from '~/analytics';
import { RemedyLine } from '~/features/company/components/molecules';
import type { AssessmentView } from '~/features/company/constants';

export type EligibilityVerdictProps = {
  assessment: AssessmentView;
  /** Analytics surface for `go_no_go_recommended`. */
  location: AnalyticsLocation;
  className?: string;
};

/**
 * The eligibility recommendation, in the one order that is allowed: lead fact →
 * verdict → changes → caveats.
 *
 * The server encodes that order once so no consumer re-decides it; this component
 * is the client half of the same commitment, and a test asserts the lead node
 * precedes the verdict node via `compareDocumentPosition`. Never reorder these
 * with CSS `order` or `flex-col-reverse` — the DOM order IS the product rule.
 *
 * Why the FACT leads: "richiede SOA OG1 III; il profilo indica OG1 I" is
 * checkable by the reader in two seconds. "No-go" is not checkable at all, and a
 * verdict that arrives before its evidence trains people either to trust it
 * blindly or to stop reading it. Both are worse than a slightly longer sentence.
 *
 * The block does NOT animate. A verdict that slides in reads as a reveal; this
 * one has to read as a record. It carries `role="status"` instead, so a re-check
 * after a captured answer is announced rather than performed.
 */
export function EligibilityVerdict({ assessment, location, className }: EligibilityVerdictProps) {
  const { t } = useTranslation();

  const gaps = assessment.gaps ?? [];
  const caveats = assessment.caveats ?? [];
  // Already sorted blocking-first by the domain. Reading [0] is not a re-sort.
  const lead = gaps[0];
  const remediable = gaps.filter((g) => (g.remedy ?? '') !== '');

  // Keyed on `evaluatedAt`: the identity of the ASSESSMENT, not of this
  // component. Answering a captured question re-runs the check and the verdict
  // may visibly flip, which is a new observation and fires again; the same
  // assessment re-rendered never does.
  useCaptureEventOnce(
    'go_no_go_recommended',
    assessment.evaluatedAt ?? `${assessment.verdict}:${gaps.length}`,
    {
      location,
      recommendation: (assessment.verdict ?? '') as ReportedVerdict,
      // Deliberately no confidence band. A single scalar would hide the one
      // distinction that decides what to build next: `insufficient_data` because
      // we read nothing, versus because the user has not answered a question.
      // caveat_count plus eligibility_gap_shown{coverage, reason} carry it losslessly.
      caveat_count: caveats.length,
      authoritative_requirement_count: assessment.authoritativeRequirementCount ?? 0,
      requirement_count: assessment.requirementCount ?? 0,
    },
  );

  return (
    <div role="status" className={cn('flex flex-col gap-4', className)}>
      {/* 1. LEAD — the blocking fact, in the gara's own words next to the
             dossier's own words. Both are quoted data; neither is translated. */}
      <p data-testid="eligibility-lead" className="text-sm text-ink-900">
        {lead?.requirement?.requirementText ? (
          <>
            <span className="text-ink-500">
              {t('company.eligibility.lead.requires', 'This gara requires')}{' '}
            </span>
            {lead.requirement.requirementText}
            {lead.held && (
              <>
                <span className="text-ink-500">
                  {' '}
                  · {t('company.eligibility.lead.holds', 'your dossier holds')}{' '}
                </span>
                {lead.held}
              </>
            )}
          </>
        ) : (
          // Not "no gaps found": nothing has been recorded for this gara, which
          // is ignorance, not a clean bill.
          t(
            'company.eligibility.lead.noRequirements',
            'No participation requirement has been recorded for this gara.',
          )
        )}
      </p>

      {/* 2. VERDICT — one localized line, never a bare pill. A pill on its own is
             a verdict with no fact behind it. */}
      <p data-testid="eligibility-verdict" className="text-sm font-semibold text-ink-900">
        {t(
          `company.eligibility.verdict.${assessment.verdict}`,
          VERDICT_FALLBACK[assessment.verdict] ?? assessment.verdict,
        )}
      </p>

      {/* 3. CHANGES — what would close each gap, in gap order. */}
      {remediable.length > 0 && (
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-ink-400">
            {t('company.eligibility.changesLabel', 'What would change this')}
          </p>
          <ul className="mt-2 flex flex-col gap-1">
            {remediable.map((gap, i) => (
              <RemedyLine key={gap.requirement?.id ?? `${gap.status}-${i}`} gap={gap} />
            ))}
          </ul>
        </div>
      )}

      {/* 4. CAVEATS — what the verdict does not know. Silence about these is how
             ignorance gets read as a clean bill. */}
      {caveats.length > 0 && (
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-ink-400">
            {t('company.eligibility.caveatsLabel', 'What this does not cover')}
          </p>
          <ul className="mt-2 flex flex-col gap-1">
            {caveats.map((caveat) => (
              <li key={caveat} className="flex items-start gap-2 text-sm text-ink-500">
                <AlertTriangle
                  aria-hidden="true"
                  strokeWidth={1.75}
                  className="mt-0.5 h-3.5 w-3.5 shrink-0"
                />
                {t(`company.eligibility.caveat.${caveat}`, CAVEAT_FALLBACK[caveat] ?? caveat)}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

const VERDICT_FALLBACK: Record<string, string> = {
  insufficient_data: 'Not enough recorded to say.',
  go: 'You appear eligible to take part.',
  no_go: 'You do not currently meet a blocking requirement.',
};

const CAVEAT_FALLBACK: Record<string, string> = {
  no_requirements_captured: 'No participation requirement has been captured for this gara.',
  award_grid_only: 'Only the award criteria were read, not the participation requirements.',
  unconfirmed_extraction: 'Some requirements were read by the agent and not yet confirmed.',
  open_questions: 'Some dossier questions are still unanswered.',
  documents_unread: 'Some of the buyer’s documents have not been read.',
};
