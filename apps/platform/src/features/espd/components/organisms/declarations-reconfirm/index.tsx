import { Banner, Card, cn } from '@tendersbay/components/core';
import type { DeclarationState } from '@tendersbay/proto/espd/v1/espd_pb';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import { criterionLabel } from '~/features/espd/components/molecules/dgue-gap-row';
import { EXCLUSION_CRITERIA } from '~/features/espd/constants';
import { espdClient } from '~/lib/api/client';

export type DeclarationsReconfirmProps = {
  workbenchId: string;
  bidId: string;
  declarations: DeclarationState;
  canManage: boolean;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Where "something has changed" goes. */
  onOpenDossier: () => void;
  /** Recompose the preview so the readiness count moves. */
  onConfirmed: () => void;
  className?: string;
};

/**
 * "You declared these on 12 March. Still true today?" — one screen, one button.
 *
 * The whole design question here was how many decisions to ask for. The honest
 * answer is 23 — one per exclusion ground — and the honest answer is wrong: a
 * person re-reading 23 criminal and insolvency grounds before every tender will
 * stop reading by the fourth. So this asks for ONE decision over a list they can
 * read, which is what the ESPD itself asks for (a single signed statement), and
 * makes changing an answer a deliberate detour into the dossier rather than an
 * inline edit next to a confirm button.
 *
 * What it must never do is make confirming easier than reading. So:
 *
 *  - Every ground is listed with its answer, not summarised as "23 answered".
 *  - The grounds that APPLY are called out first and marked, because those are
 *    the ones with consequences and the ones a stale confirmation would carry
 *    forward silently.
 *  - The confirmation date and the person are shown before the button, not after.
 */
export function DeclarationsReconfirm({
  workbenchId,
  bidId,
  declarations,
  canManage,
  location,
  onOpenDossier,
  onConfirmed,
  className,
}: DeclarationsReconfirmProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const byCriterion = new Map(declarations.answers.map((a) => [a.criterion, a]));
  // Driven by OUR closed list rather than by what the server returned: a ground
  // the server stopped sending would silently vanish from the screen someone is
  // about to sign, which is the one place a silent omission is unacceptable.
  const rows = EXCLUSION_CRITERIA.map((criterion) => ({
    criterion,
    answer: byCriterion.get(criterion),
  }));
  const applies = rows.filter((r) => r.answer?.applies);
  const stale = declarations.complete && !declarations.confirmed;

  async function confirm() {
    setError(null);
    setSaving(true);
    try {
      await espdClient.confirmDeclarations({ workbenchId, bidId });
      capture('part_iii_reconfirmed', {
        location,
        declaration_count: declarations.answers.length,
        applies_count: applies.length,
        // The first confirmation and a re-confirmation after a change are
        // different acts: the second is the friction this screen exists to
        // measure, and a single count cannot tell them apart.
        was_stale: stale && Boolean(declarations.confirmedAt),
      });
      onConfirmed();
    } catch (e: unknown) {
      setError(
        e instanceof Error
          ? e.message
          : t('espd.declarations.error', 'Could not confirm — try again.'),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card className={className}>
      <h2 className="font-semibold text-ink-700 text-sm">
        {t('espd.declarations.title', 'Part III — exclusion grounds')}
      </h2>

      {!declarations.complete ? (
        <>
          <p className="mt-2 text-ink-500 text-sm">
            {t(
              'espd.declarations.incomplete',
              'Some grounds have never been answered. Answer them in the company dossier — once, for every tender.',
            )}
          </p>
          <button type="button" onClick={onOpenDossier} className={cn(SECONDARY_BUTTON, 'mt-3')}>
            {t('espd.declarations.openDossier', 'Open the dossier')}
          </button>
        </>
      ) : (
        <>
          <p className="mt-2 text-ink-600 text-sm">
            {declarations.confirmedAt
              ? t('espd.declarations.declaredOn', {
                  when: formatDate(declarations.confirmedAt),
                  defaultValue: 'Confirmed for this tender on {{when}}.',
                })
              : t(
                  'espd.declarations.neverConfirmed',
                  'These answers have not been confirmed for this tender yet.',
                )}
          </p>

          {stale && (
            <Banner tone="warning" className="mt-3">
              {t(
                'espd.declarations.stale',
                'An answer changed after the last confirmation. Read them again and confirm.',
              )}
            </Banner>
          )}

          {applies.length > 0 && (
            <div className="mt-4">
              <p className="font-semibold text-ink-700 text-xs uppercase tracking-wide">
                {t('espd.declarations.appliesTitle', 'Grounds you declared as applying')}
              </p>
              <ul className="mt-2 flex list-none flex-col gap-2 p-0">
                {applies.map(({ criterion, answer }) => (
                  <li key={criterion} className="rounded-xl bg-cream-100 p-3">
                    <p className="font-medium text-ink-900 text-sm">
                      {criterionLabel(criterion, t)}
                    </p>
                    {answer?.selfCleaning && (
                      <p className="mt-1 text-ink-600 text-xs">
                        {t('espd.declarations.selfCleaning', 'Measures taken')}:{' '}
                        {answer.selfCleaning}
                      </p>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}

          <details className="mt-4">
            <summary className="cursor-pointer font-semibold text-ink-700 text-xs uppercase tracking-wide">
              {t('espd.declarations.allTitle', 'All grounds')}
            </summary>
            <ul className="mt-2 flex list-none flex-col p-0">
              {rows.map(({ criterion, answer }) => (
                <li
                  key={criterion}
                  className="flex items-baseline justify-between gap-3 border-cream-200 border-b py-1.5 last:border-b-0"
                >
                  <span className="text-ink-700 text-sm">{criterionLabel(criterion, t)}</span>
                  <span className="shrink-0 text-ink-500 text-xs">
                    {answer?.answered
                      ? answer.applies
                        ? t('espd.declarations.applies', 'Applies')
                        : t('espd.declarations.doesNotApply', 'Does not apply')
                      : t('espd.declarations.unanswered', 'Unanswered')}
                  </span>
                </li>
              ))}
            </ul>
          </details>

          {error && (
            <Banner tone="error" className="mt-3">
              {error}
            </Banner>
          )}

          {canManage && (
            <div className="mt-4 flex flex-wrap items-center gap-2">
              <button
                type="button"
                disabled={saving}
                onClick={() => void confirm()}
                className={cn(PRIMARY_BUTTON, saving && 'opacity-50')}
              >
                {saving
                  ? t('espd.declarations.confirming', 'Confirming…')
                  : t('espd.declarations.confirm', 'Yes — still true for this tender')}
              </button>
              <button type="button" onClick={onOpenDossier} className={SECONDARY_BUTTON}>
                {t('espd.declarations.changed', 'Something has changed')}
              </button>
            </div>
          )}
        </>
      )}
    </Card>
  );
}

/**
 * RFC3339 to the reader's own locale. `undefined` as the locale hands it to the
 * browser's, which is what a date in a legal statement should follow — the app's
 * UI language is not necessarily the reader's date convention.
 */
function formatDate(rfc3339: string): string {
  const parsed = new Date(rfc3339);
  if (Number.isNaN(parsed.getTime())) return rfc3339;
  return parsed.toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' });
}

const PRIMARY_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl bg-brand-600 px-4 font-semibold text-sm text-white ' +
  'outline-none transition-colors duration-150 hover:bg-brand-700 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2';

const SECONDARY_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl border border-cream-300 px-4 font-semibold text-ink-700 text-sm ' +
  'outline-none transition-colors duration-150 hover:bg-cream-200 focus-visible:ring-2 focus-visible:ring-brand-600';
