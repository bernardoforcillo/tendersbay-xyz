import { Banner, Card, cn } from '@tendersbay/components/core';
import type { CompanyDossier, Declaration } from '@tendersbay/proto/company/v1/company_pb';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import { ProvenanceMark } from '~/features/company/components/atoms';
import {
  criterionLabel,
  DECLARATION_PARTS,
  EXCLUSION_CRITERIA,
  partOfCriterion,
} from '~/features/espd';
import { companyClient } from '~/lib/api/client';

export type DeclarationsFormProps = {
  workspaceId: string;
  dossier: CompanyDossier;
  canManage: boolean;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Reload the dossier so an answer visibly lands. */
  onSaved: () => void;
  className?: string;
};

/**
 * Part III of the DGUE, in the dossier: the twenty-three exclusion grounds,
 * answered once and reused for every tender.
 *
 * This is the only form in the product where the answer is a legal declaration
 * rather than a fact, and it is built differently because of it:
 *
 *  - **The default is nothing.** An unanswered ground shows as unanswered, never
 *    as "no". Pre-filling twenty-three "no"s would be the product declaring on
 *    the operator's behalf that it has no convictions — which is precisely the
 *    statement they are criminally liable for.
 *  - **The answer is two explicit buttons**, not a checkbox. A checkbox has an
 *    unchecked state that reads as "no" and means "not touched".
 *  - **Self-cleaning appears only when a ground applies**, because Art. 57(6)
 *    measures are meaningless otherwise — and the server refuses them there.
 *  - **Provenance is visible on every row.** The server records these as stated
 *    by the person who typed them and refuses anything else; showing who and
 *    when is what makes the per-tender re-confirmation mean something.
 */
export function DeclarationsForm({
  workspaceId,
  dossier,
  canManage,
  location,
  onSaved,
  className,
}: DeclarationsFormProps) {
  const { t } = useTranslation();

  const byCriterion = new Map(dossier.declarations.map((d) => [d.criterion, d]));
  const answered = EXCLUSION_CRITERIA.filter((c) => byCriterion.has(c)).length;

  return (
    <Card className={className}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="font-semibold text-ink-700 text-sm">
          {t('espd.dossier.title', 'Exclusion grounds (DGUE Part III)')}
        </h2>
        <span className="text-ink-500 text-xs">
          {t('espd.dossier.progress', {
            answered,
            total: EXCLUSION_CRITERIA.length,
            defaultValue: '{{answered}} of {{total}} answered',
          })}
        </span>
      </div>
      <p className="mt-1 text-ink-500 text-sm">
        {t(
          'espd.dossier.intro',
          'Answer these once. Every tender reuses them — you only re-confirm that they are still true.',
        )}
      </p>

      {/*
        Grouped under the same three headings the official form carries, in the
        Union's own wording. Twenty-three grounds in one flat list is a wall;
        three sections of six to thirteen is the document the operator has
        already seen on the contracting authority's portal.
      */}
      {DECLARATION_PARTS.map((part) => (
        <section key={part} className="mt-5">
          <h3 className="font-semibold text-ink-500 text-xs uppercase tracking-wide">
            {t(`espd.parts.${part}`, part)}
          </h3>
          <div className="mt-2 flex flex-col gap-2">
            {EXCLUSION_CRITERIA.filter((c) => partOfCriterion(c) === part).map((criterion) => (
              <DeclarationRow
                key={criterion}
                workspaceId={workspaceId}
                criterion={criterion}
                declaration={byCriterion.get(criterion)}
                canManage={canManage}
                location={location}
                onSaved={onSaved}
              />
            ))}
          </div>
        </section>
      ))}
    </Card>
  );
}

function DeclarationRow({
  workspaceId,
  criterion,
  declaration,
  canManage,
  location,
  onSaved,
}: {
  workspaceId: string;
  criterion: string;
  declaration?: Declaration;
  canManage: boolean;
  location: AnalyticsLocation;
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();

  // The self-cleaning draft is local and unsaved until the row is saved, like
  // every other draft in this app. Nothing here is persisted on keystroke: a
  // half-typed description of a criminal conviction is not a fact.
  const [selfCleaning, setSelfCleaning] = useState(declaration?.selfCleaning ?? '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const answered = declaration !== undefined;
  const applies = declaration?.answer ?? false;

  async function save(nextApplies: boolean, nextSelfCleaning: string) {
    setError(null);
    setSaving(true);
    try {
      await companyClient.putDeclaration({
        workspaceId,
        declaration: {
          criterion,
          answer: nextApplies,
          // A ground that does not apply carries no measures — the server
          // rejects the pair outright, so it is not sent.
          selfCleaning: nextApplies ? nextSelfCleaning : '',
          // No provenance is sent: the server derives it from how the call
          // arrived and refuses anything but a human statement here.
          attribution: { promptedBy: 'onboarding', sourceNote: '' },
        },
      });
      capture('dgue_gap_filled', {
        location,
        scope: 'company',
        reason: answered ? 'stale' : 'missing',
      });
      onSaved();
    } catch (e: unknown) {
      setError(
        e instanceof Error ? e.message : t('espd.dossier.error', 'Could not save — try again.'),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="rounded-xl border border-cream-200 p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <p className="text-ink-900 text-sm">{criterionLabel(criterion, t)}</p>
        {answered && (
          <ProvenanceMark
            attribution={declaration?.attribution}
            detail={
              declaration?.attribution?.statedAt
                ? t('espd.dossier.statedAt', {
                    when: formatDate(declaration.attribution.statedAt),
                    defaultValue: 'Declared on {{when}}',
                  })
                : undefined
            }
          />
        )}
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-2">
        <AnswerButton
          label={t('espd.dossier.doesNotApply', 'Does not apply to us')}
          selected={answered && !applies}
          disabled={!canManage || saving}
          onPress={() => void save(false, '')}
        />
        <AnswerButton
          label={t('espd.dossier.applies', 'Applies to us')}
          selected={answered && applies}
          disabled={!canManage || saving}
          onPress={() => void save(true, selfCleaning)}
        />
        {!answered && (
          <span className="text-ink-400 text-xs">
            {t('espd.dossier.unanswered', 'Not answered yet')}
          </span>
        )}
      </div>

      {answered && applies && canManage && (
        <div className="mt-3">
          <label className="flex flex-col gap-1.5 font-medium text-ink-700 text-sm">
            {t('espd.dossier.selfCleaning', 'Measures taken (self-cleaning)')}
            <textarea
              rows={3}
              value={selfCleaning}
              onChange={(e) => setSelfCleaning(e.target.value)}
              onBlur={() => {
                if (selfCleaning !== (declaration?.selfCleaning ?? ''))
                  void save(true, selfCleaning);
              }}
              className="w-full rounded-xl border border-cream-300 bg-white px-3 py-2 text-ink-900 text-sm outline-none transition-colors duration-150 placeholder:text-ink-300 focus:border-brand-600 focus:ring-2 focus:ring-brand-600/25"
            />
          </label>
          <p className="mt-1 text-ink-400 text-xs">
            {t(
              'espd.dossier.selfCleaningHint',
              'What you did about it — Article 57(6). A contracting authority reads this before deciding whether to exclude you.',
            )}
          </p>
        </div>
      )}

      {error && (
        <Banner tone="error" className="mt-2">
          {error}
        </Banner>
      )}
    </div>
  );
}

function AnswerButton({
  label,
  selected,
  disabled,
  onPress,
}: {
  label: string;
  selected: boolean;
  disabled: boolean;
  onPress: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      disabled={disabled}
      onClick={onPress}
      className={cn(
        'rounded-full border px-3 py-1.5 font-medium text-xs outline-none transition-colors focus-visible:ring-2 focus-visible:ring-brand-600',
        selected
          ? 'border-brand-600 bg-brand-100 text-brand-700'
          : 'border-cream-300 bg-white text-ink-600 hover:border-cream-400',
        disabled && 'cursor-default opacity-50',
      )}
    >
      {label}
    </button>
  );
}

function formatDate(rfc3339: string): string {
  const parsed = new Date(rfc3339);
  if (Number.isNaN(parsed.getTime())) return rfc3339;
  return parsed.toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' });
}
