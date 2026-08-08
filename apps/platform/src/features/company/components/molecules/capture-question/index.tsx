import { Banner, cn } from '@tendersbay/components/core';
import { useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation, ReportedFieldCategory } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import {
  CERTIFICATION_STANDARDS,
  PAST_CONTRACT_ROLES,
  REGISTRATION_KINDS,
  type RequirementView,
  SOA_CATEGORIES,
  SOA_CLASSIFICHE,
} from '~/features/company/constants';
import { EvidenceQuote } from '~/features/tenders';
import { citationFromRequirement } from '~/features/tenders/constants';
import { companyClient } from '~/lib/api/client';

export type CaptureQuestionProps = {
  workspaceId: string;
  /** Stringified tender id — stamped on every write as the prompt's origin. */
  tenderId: string;
  requirement: RequirementView;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Re-run the eligibility check so the answer visibly changes the verdict. */
  onSaved: () => void;
  className?: string;
};

/**
 * The just-in-time dossier question: asked in the gap row, on the scheda gara, at
 * the moment the go/no-go actually needs the answer.
 *
 * Not a wizard, not a settings page, not a modal. Up-front dossier capture is
 * maximum friction at zero demonstrated value; here the user has already seen a
 * gara they care about and a specific thing standing between them and a verdict,
 * so the question has earned its keystrokes. It renders ALREADY EXPANDED for the
 * same reason: an unknown gap *is* the question, and hiding it behind a click
 * adds a step to the one interaction that pays for itself.
 *
 * Three things about what it shows and sends:
 *
 *  - The published requirement text and its quoted passage sit above the
 *    controls. The user must be able to check what we are asking about against
 *    the source before answering — verification cost below doubt, here too.
 *  - The prompt is the UI's own localized string keyed by `requirement.kind`.
 *    The server's `question` is Italian model-facing prose and is never rendered;
 *    a 24-locale product builds its own.
 *  - Every write carries `attribution.promptedBy = 'tender'` and the tender id,
 *    and deliberately NO provenance/statedBy/statedAt. The server derives those
 *    from how the call arrived and ignores whatever a client sends, so a client
 *    has no lever with which to forge a stronger claim than the one it made.
 */
export function CaptureQuestion({
  workspaceId,
  tenderId,
  requirement,
  location,
  onSaved,
  className,
}: CaptureQuestionProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();

  // Every draft field is component-local and nothing here is persisted — not even
  // the "Not sure" dismissal. An unanswered question MUST come back next visit,
  // because `unknown` is what triggers the ask in the first place.
  const [dismissed, setDismissed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft>(EMPTY_DRAFT);

  const kind = requirement.kind;
  const attribution = { promptedBy: 'tender', promptedByTenderId: tenderId, sourceNote: '' };

  function set<K extends keyof Draft>(key: K, value: Draft[K]) {
    setDraft((d) => ({ ...d, [key]: value }));
  }

  if (dismissed) return null;

  async function save() {
    setError(null);
    setSaving(true);
    try {
      await write(kind, workspaceId, draft, attribution);
      capture('company_dossier_field_answered', {
        location,
        field_category: kind as ReportedFieldCategory,
        // The scheda gara arm of the PRD's central hypothesis. The settings arm
        // is the dossier form's, and the two are only comparable because this
        // one property is a closed set rather than whatever string a caller
        // happened to pass down.
        prompted_by: 'tender',
      });
      onSaved();
    } catch (e: unknown) {
      setError(
        e instanceof Error
          ? e.message
          : t('company.capture.error', 'Could not save that — try again.'),
      );
    } finally {
      setSaving(false);
    }
  }

  function skip() {
    // Collapses for THIS render only. Nothing is written to the dossier: "not
    // sure" is not an answer, and recording it as one would silence the question
    // forever on the strength of a shrug.
    setDismissed(true);
    capture('company_dossier_question_skipped', {
      location,
      field_category: kind as ReportedFieldCategory,
    });
  }

  const citation = citationFromRequirement(requirement);

  return (
    <div className={cn('flex flex-col gap-3 rounded-xl bg-cream-50 p-3', className)}>
      <div className="flex flex-col gap-2">
        <p className="text-sm text-ink-900">{requirement.requirementText}</p>
        {requirement.excerpt && (
          <EvidenceQuote
            tenderId={tenderId}
            excerpt={requirement.excerpt}
            citation={citation}
            location={location}
          />
        )}
      </div>

      <p className="text-sm font-medium text-ink-700">
        {t(
          `company.capture.prompt.${kind}`,
          PROMPT_FALLBACK[kind] ?? PROMPT_FALLBACK.other ?? kind,
        )}
      </p>

      {kind === 'soa' && (
        <>
          <ChipGroup
            label={t('company.dossier.soa.category', 'Category')}
            options={SOA_CATEGORIES}
            selected={draft.category ? [draft.category] : []}
            onToggle={(v) => set('category', draft.category === v ? '' : v)}
          />
          <ChipGroup
            label={t('company.dossier.soa.classifica', 'Classifica')}
            options={[...SOA_CLASSIFICHE]}
            selected={draft.classifica ? [draft.classifica] : []}
            onToggle={(v) => set('classifica', draft.classifica === v ? '' : v)}
          />
          {/* The triennial verification lapses three years before validity and
              invalidates the attestation on its own, so asking only for
              validUntil would record a lapsed SOA as valid for two more years. */}
          <TextInput
            label={t('company.dossier.soa.verifiedUntil', 'Verified until')}
            hint={t(
              'company.dossier.soa.verifiedUntilHint',
              'The triennial verification date — it lapses before the attestation itself.',
            )}
            type="date"
            value={draft.verifiedUntil}
            onChange={(v) => set('verifiedUntil', v)}
          />
        </>
      )}

      {kind === 'certification' && (
        <>
          <ChipGroup
            label={t('company.dossier.certifications.standard', 'Standard')}
            options={[...CERTIFICATION_STANDARDS]}
            selected={draft.standard ? [draft.standard] : []}
            onToggle={(v) => set('standard', draft.standard === v ? '' : v)}
            translateKey="company.dossier.certifications.standards"
            labels={CERTIFICATION_LABELS}
          />
          {draft.standard === 'other' && (
            <TextInput
              label={t('company.dossier.certifications.standardRaw', 'Published name')}
              value={draft.standardRaw}
              onChange={(v) => set('standardRaw', v)}
            />
          )}
          <TextInput
            label={t('company.dossier.certifications.scope', 'Certified scope')}
            value={draft.scope}
            onChange={(v) => set('scope', v)}
          />
          <TextInput
            label={t('company.dossier.certifications.validUntil', 'Valid until')}
            type="date"
            value={draft.validUntil}
            onChange={(v) => set('validUntil', v)}
          />
        </>
      )}

      {kind === 'turnover' && (
        <>
          <TextInput
            label={t('company.dossier.financialYears.year', 'Year')}
            type="number"
            value={draft.year}
            onChange={(v) => set('year', v)}
          />
          <TextInput
            label={t('company.dossier.financialYears.turnover', 'Turnover')}
            type="number"
            value={draft.turnover}
            onChange={(v) => set('turnover', v)}
          />
          <TextInput
            label={t('company.dossier.financialYears.currency', 'Currency')}
            value={draft.currency}
            onChange={(v) => set('currency', v.toUpperCase())}
          />
        </>
      )}

      {kind === 'headcount' && (
        <>
          <TextInput
            label={t('company.dossier.financialYears.year', 'Year')}
            type="number"
            value={draft.year}
            onChange={(v) => set('year', v)}
          />
          <TextInput
            label={t('company.dossier.financialYears.headcount', 'Headcount')}
            type="number"
            value={draft.headcount}
            onChange={(v) => set('headcount', v)}
          />
        </>
      )}

      {kind === 'past_contracts' && (
        <>
          <TextInput
            label={t('company.dossier.pastContracts.description', 'What the contract was')}
            value={draft.description}
            onChange={(v) => set('description', v)}
          />
          <TextInput
            label={t('company.dossier.pastContracts.buyer', 'Buyer')}
            value={draft.buyerName}
            onChange={(v) => set('buyerName', v)}
          />
          <TextInput
            label={t('company.dossier.pastContracts.cpv', 'CPV')}
            value={draft.cpv}
            onChange={(v) => set('cpv', v)}
          />
          <TextInput
            label={t('company.dossier.pastContracts.value', 'Value')}
            type="number"
            value={draft.value}
            onChange={(v) => set('value', v)}
          />
          <TextInput
            label={t('company.dossier.pastContracts.started', 'Start date')}
            type="date"
            value={draft.startedOn}
            onChange={(v) => set('startedOn', v)}
          />
          <TextInput
            label={t('company.dossier.pastContracts.ended', 'End date')}
            type="date"
            value={draft.endedOn}
            onChange={(v) => set('endedOn', v)}
          />
          <ChipGroup
            label={t('company.dossier.pastContracts.role', 'Role')}
            options={[...PAST_CONTRACT_ROLES]}
            selected={draft.role ? [draft.role] : []}
            onToggle={(v) => set('role', draft.role === v ? '' : v)}
            translateKey="company.dossier.pastContracts.roles"
            labels={ROLE_LABELS}
          />
          {/* A 20% mandante on a €5M contract carries €1M of reference, not €5M —
              the share is what actually counts toward a requirement. */}
          <TextInput
            label={t('company.dossier.pastContracts.sharePct', 'Your share (%)')}
            hint={t(
              'company.dossier.pastContracts.sharePctHint',
              'Only your own share counts toward a requirement.',
            )}
            type="number"
            value={draft.sharePct}
            onChange={(v) => set('sharePct', v)}
          />
        </>
      )}

      {kind === 'registration' && (
        <>
          <ChipGroup
            label={t('company.dossier.registrations.kind', 'Register')}
            options={[...REGISTRATION_KINDS]}
            selected={draft.registrationKind ? [draft.registrationKind] : []}
            onToggle={(v) => set('registrationKind', draft.registrationKind === v ? '' : v)}
            translateKey="company.dossier.registrations.kinds"
            labels={REGISTRATION_LABELS}
          />
          <TextInput
            label={t('company.dossier.registrations.authority', 'Authority')}
            value={draft.authority}
            onChange={(v) => set('authority', v)}
          />
          <TextInput
            label={t('company.dossier.registrations.section', 'Section')}
            value={draft.section}
            onChange={(v) => set('section', v)}
          />
          <TextInput
            label={t('company.dossier.registrations.validUntil', 'Valid until')}
            type="date"
            value={draft.validUntil}
            onChange={(v) => set('validUntil', v)}
          />
        </>
      )}

      {kind === 'other' && (
        <p className="text-sm text-ink-500">
          {t(
            'company.capture.cannotCheck',
            'We cannot check this one automatically — read the requirement and decide.',
          )}
        </p>
      )}

      {error && <Banner tone="error">{error}</Banner>}

      <div className="flex flex-wrap items-center gap-2">
        {kind === 'other' ? (
          // Disabled-but-focusable: `isDisabled` would drop the control from the
          // tab order and take its explanation with it. There is genuinely
          // nothing to write for an unclassified requirement.
          <Button
            aria-disabled="true"
            onPress={() => {}}
            className={cn(SAVE_BUTTON, 'cursor-default opacity-50')}
          >
            {t('company.capture.save', 'Save')}
          </Button>
        ) : (
          <Button
            isDisabled={saving}
            onPress={() => void save()}
            className={cn(SAVE_BUTTON, saving && 'opacity-50')}
          >
            {saving ? t('company.capture.saving', 'Saving…') : t('company.capture.save', 'Save')}
          </Button>
        )}
        <Button onPress={skip} className={SKIP_BUTTON}>
          {t('company.capture.notSure', 'Not sure')}
        </Button>
      </div>
    </div>
  );
}

const SAVE_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl bg-brand-600 px-4 text-sm font-semibold text-white outline-none ' +
  'transition-colors duration-150 data-[hovered]:bg-brand-700 data-[pressed]:bg-brand-800 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2';

const SKIP_BUTTON =
  'inline-flex h-10 items-center justify-center rounded-xl px-4 text-sm font-semibold text-ink-700 outline-none ' +
  'transition-colors duration-150 data-[hovered]:bg-cream-200 data-[pressed]:bg-cream-300 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

const CHIP_BASE =
  'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors outline-none ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';
const CHIP_ON = 'border-brand-600 bg-brand-100 text-brand-700';
const CHIP_OFF = 'border-cream-300 bg-white text-ink-600 data-[hovered]:border-cream-400';

const TEXT_INPUT =
  'h-10 w-full rounded-xl border border-cream-300 bg-white px-3 text-sm text-ink-900 outline-none ' +
  'transition-colors duration-150 placeholder:text-ink-300 focus:border-brand-600 focus:ring-2 focus:ring-brand-600/25';

/**
 * A closed set rendered as chips. The kit has no chip primitive and this is one
 * feature's affordance, so it stays local rather than widening the shared kit —
 * the same call `client-profile-form` made for the same reason.
 */
function ChipGroup({
  label,
  options,
  selected,
  onToggle,
  translateKey,
  labels,
}: {
  label: string;
  options: string[];
  selected: string[];
  onToggle: (value: string) => void;
  /** When set, each option is localized as `<translateKey>.<option>`. */
  translateKey?: string;
  /**
   * English fallbacks per option. A closed-set code is a wire constant, not copy —
   * without a fallback an untranslated chip would read "albo_gestori_ambientali".
   */
  labels?: Record<string, string>;
}) {
  const { t } = useTranslation();
  return (
    <div>
      <p className="mb-2 text-sm font-medium text-ink-700">{label}</p>
      <div className="flex max-h-32 flex-wrap gap-2 overflow-y-auto">
        {options.map((option) => (
          <Button
            key={option}
            aria-pressed={selected.includes(option)}
            onPress={() => onToggle(option)}
            className={cn(CHIP_BASE, selected.includes(option) ? CHIP_ON : CHIP_OFF)}
          >
            {translateKey ? t(`${translateKey}.${option}`, labels?.[option] ?? option) : option}
          </Button>
        ))}
      </div>
    </div>
  );
}

function TextInput({
  label,
  hint,
  type = 'text',
  value,
  onChange,
}: {
  label: string;
  hint?: string;
  type?: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="flex flex-col gap-1.5 text-sm font-medium text-ink-700">
        {label}
        <input
          type={type}
          className={TEXT_INPUT}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      </label>
      {hint && <p className="text-xs text-ink-400">{hint}</p>}
    </div>
  );
}

type Draft = {
  category: string;
  classifica: string;
  verifiedUntil: string;
  standard: string;
  standardRaw: string;
  scope: string;
  validUntil: string;
  year: string;
  turnover: string;
  currency: string;
  headcount: string;
  description: string;
  buyerName: string;
  cpv: string;
  value: string;
  startedOn: string;
  endedOn: string;
  role: string;
  sharePct: string;
  registrationKind: string;
  authority: string;
  section: string;
};

const EMPTY_DRAFT: Draft = {
  category: '',
  classifica: '',
  verifiedUntil: '',
  standard: '',
  standardRaw: '',
  scope: '',
  validUntil: '',
  year: '',
  turnover: '',
  currency: 'EUR',
  headcount: '',
  description: '',
  buyerName: '',
  cpv: '',
  value: '',
  startedOn: '',
  endedOn: '',
  role: '',
  sharePct: '',
  registrationKind: '',
  authority: '',
  section: '',
};

type AttributionInit = { promptedBy: string; promptedByTenderId: string; sourceNote: string };

/** Money travels as int64 minor units; a blank field means "not asserted", not zero. */
function minor(amount: string): { value: bigint; set: boolean } {
  const trimmed = amount.trim();
  if (!trimmed) return { value: 0n, set: false };
  const parsed = Number.parseFloat(trimmed.replace(',', '.'));
  if (!Number.isFinite(parsed)) return { value: 0n, set: false };
  return { value: BigInt(Math.round(parsed * 100)), set: true };
}

function int(value: string): number {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) ? parsed : 0;
}

/**
 * Routes one answer to the one dossier RPC that owns it.
 *
 * `turnover` and `headcount` both write a FinancialYear: Italian selection
 * criteria ask for both per esercizio ("fatturato globale degli ultimi tre
 * esercizi", "organico medio annuo del triennio"), so the record carries both and
 * each write asserts only the half it was asked about.
 */
async function write(
  kind: string,
  workspaceId: string,
  draft: Draft,
  attribution: AttributionInit,
): Promise<void> {
  switch (kind) {
    case 'soa':
      await companyClient.putSoaCategory({
        workspaceId,
        soa: {
          category: draft.category,
          classifica: draft.classifica,
          verifiedUntil: draft.verifiedUntil,
          attribution,
        },
      });
      return;
    case 'certification':
      await companyClient.putCertification({
        workspaceId,
        certification: {
          standard: draft.standard,
          standardRaw: draft.standardRaw,
          scope: draft.scope,
          validUntil: draft.validUntil,
          attribution,
        },
      });
      return;
    case 'turnover': {
      const turnover = minor(draft.turnover);
      await companyClient.putFinancialYear({
        workspaceId,
        financialYear: {
          year: int(draft.year),
          turnoverMinor: turnover.value,
          turnoverSet: turnover.set,
          currency: draft.currency,
          attribution,
        },
      });
      return;
    }
    case 'headcount':
      await companyClient.putFinancialYear({
        workspaceId,
        financialYear: {
          year: int(draft.year),
          headcount: int(draft.headcount),
          headcountSet: draft.headcount.trim() !== '',
          attribution,
        },
      });
      return;
    case 'past_contracts': {
      const value = minor(draft.value);
      const share = Number.parseFloat(draft.sharePct.trim().replace(',', '.'));
      await companyClient.putPastContract({
        workspaceId,
        pastContract: {
          description: draft.description,
          buyerName: draft.buyerName,
          cpv: draft.cpv,
          valueMinor: value.value,
          valueSet: value.set,
          currency: draft.currency,
          startedOn: draft.startedOn,
          endedOn: draft.endedOn,
          role: draft.role,
          sharePct: Number.isFinite(share) ? share : 0,
          sharePctSet: Number.isFinite(share),
          attribution,
        },
      });
      return;
    }
    case 'registration':
      await companyClient.putRegistration({
        workspaceId,
        registration: {
          kind: draft.registrationKind,
          authority: draft.authority,
          section: draft.section,
          validUntil: draft.validUntil,
          attribution,
        },
      });
      return;
    default:
      // `other` has no structured target by design — there is nothing to write.
      return;
  }
}

const CERTIFICATION_LABELS: Record<string, string> = {
  iso_9001: 'ISO 9001 — quality',
  iso_14001: 'ISO 14001 — environment',
  iso_45001: 'ISO 45001 — health & safety',
  iso_27001: 'ISO 27001 — information security',
  iso_37001: 'ISO 37001 — anti-bribery',
  iso_50001: 'ISO 50001 — energy',
  sa_8000: 'SA 8000 — social accountability',
  uni_pdr_125: 'UNI/PdR 125 — gender equality',
  iso_20121: 'ISO 20121 — event sustainability',
  emas: 'EMAS',
  other: 'Other',
};

const ROLE_LABELS: Record<string, string> = {
  principal: 'Principal contractor',
  subcontractor: 'Subcontractor',
  rti_mandataria: 'RTI lead (mandataria)',
  rti_mandante: 'RTI member (mandante)',
};

const REGISTRATION_LABELS: Record<string, string> = {
  white_list: 'Prefettura white list',
  cciaa: 'Chamber of commerce (CCIAA)',
  albo_gestori_ambientali: 'Albo gestori ambientali',
  mepa: 'MePA',
  sistema_qualificazione: 'Qualification system',
  albo_professionale: 'Professional register',
  anac: 'ANAC',
  other: 'Other',
};

const PROMPT_FALLBACK: Record<string, string> = {
  soa: 'Which SOA category and classifica do you hold?',
  certification: 'Which certification do you hold?',
  turnover: 'What was your turnover?',
  past_contracts: 'Which past contract covers this?',
  headcount: 'How many people did you employ?',
  registration: 'Which register are you enrolled in?',
  other: 'We cannot check this one automatically.',
};
