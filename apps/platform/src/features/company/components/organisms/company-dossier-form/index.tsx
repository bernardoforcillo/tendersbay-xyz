import { Banner, Card, cn } from '@tendersbay/components/core';
import type {
  Certification,
  CompanyDossier,
  FinancialYear,
  PastContract,
  Registration,
  SoaCategory,
} from '@tendersbay/proto/company/v1/company_pb';
import { type ReactNode, useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import { ProvenanceMark } from '~/features/company/components/atoms';
import {
  CERTIFICATION_STANDARDS,
  LEGAL_FORMS,
  PAST_CONTRACT_ROLES,
  REGISTRATION_KINDS,
  SOA_CATEGORIES,
  SOA_CLASSIFICHE,
} from '~/features/company/constants';
import { companyClient } from '~/lib/api/client';

export type CompanyDossierFormProps = {
  workspaceId: string;
  dossier: CompanyDossier | null;
  /** Refetch the dossier after any write. */
  onChanged: () => void;
  /** Analytics surface. */
  location?: AnalyticsLocation;
};

/**
 * The company dossier's CORRECTION surface.
 *
 * Nothing in the product routes a blocked user here to unblock a gap — that is
 * the whole point of just-in-time capture. Up-front dossier entry is maximum
 * friction at zero demonstrated value, so this page exists for the user who has
 * already decided to fix something: a lapsed SOA verification, a turnover figure
 * they mistyped, an identity field a DGUE Part II will read straight off.
 *
 * Every record carries its provenance mark, because the point of a correction
 * surface is knowing which facts a human vouched for and which the agent guessed.
 *
 * The kit has no chip or repeating-record primitive and this is one feature's
 * form, so both stay local, styled to the kit's tone system — the same call
 * `client-profile-form` made.
 */
export function CompanyDossierForm({
  workspaceId,
  dossier,
  onChanged,
  location = 'company_settings',
}: CompanyDossierFormProps) {
  const { t } = useTranslation();
  const [error, setError] = useState<string | null>(null);

  return (
    <div className="flex flex-col gap-6">
      {error && <Banner tone="error">{error}</Banner>}

      <IdentitySection
        workspaceId={workspaceId}
        dossier={dossier}
        location={location}
        onChanged={onChanged}
        onError={setError}
      />

      <Section title={t('company.dossier.soa.title', 'SOA attestations')}>
        <RecordList
          empty={t('company.dossier.soa.empty', 'No SOA attestation recorded.')}
          records={dossier?.soa ?? []}
          keyOf={(soa: SoaCategory) => soa.id}
          renderLabel={(soa: SoaCategory) => `${soa.category} — ${soa.classifica}`}
          renderDetail={(soa: SoaCategory) =>
            detailLine(soa.issuedBy, soa.validUntil, soa.verifiedUntil)
          }
          attributionOf={(soa: SoaCategory) => soa.attribution}
          onRemove={(soa: SoaCategory) =>
            companyClient.removeSoaCategory({ workspaceId, id: soa.id }).then(onChanged)
          }
          onError={setError}
        />
        <SoaAdd
          workspaceId={workspaceId}
          location={location}
          onChanged={onChanged}
          onError={setError}
        />
      </Section>

      <Section title={t('company.dossier.certifications.title', 'Certifications')}>
        <RecordList
          empty={t('company.dossier.certifications.empty', 'No certification recorded.')}
          records={dossier?.certifications ?? []}
          keyOf={(c: Certification) => c.id}
          renderLabel={(c: Certification) => c.standardRaw || c.standard}
          renderDetail={(c: Certification) => detailLine(c.scope, c.issuedBy, c.validUntil)}
          attributionOf={(c: Certification) => c.attribution}
          onRemove={(c: Certification) =>
            companyClient.removeCertification({ workspaceId, id: c.id }).then(onChanged)
          }
          onError={setError}
        />
        <CertificationAdd
          workspaceId={workspaceId}
          location={location}
          onChanged={onChanged}
          onError={setError}
        />
      </Section>

      <Section title={t('company.dossier.financialYears.title', 'Financial years')}>
        <RecordList
          empty={t('company.dossier.financialYears.empty', 'No financial year recorded.')}
          records={dossier?.financialYears ?? []}
          // The year IS the identity of a financial-year record — that is why the
          // remove RPC is keyed by year and not by an id.
          keyOf={(f: FinancialYear) => String(f.year)}
          renderLabel={(f: FinancialYear) => String(f.year)}
          renderDetail={(f: FinancialYear) =>
            detailLine(
              f.turnoverSet ? `${major(f.turnoverMinor)} ${f.currency}` : '',
              f.specificTurnoverSet ? `${major(f.specificTurnoverMinor)} ${f.currency}` : '',
              f.headcountSet ? String(f.headcount) : '',
            )
          }
          attributionOf={(f: FinancialYear) => f.attribution}
          onRemove={(f: FinancialYear) =>
            companyClient.removeFinancialYear({ workspaceId, year: f.year }).then(onChanged)
          }
          onError={setError}
        />
        <FinancialYearAdd
          workspaceId={workspaceId}
          location={location}
          onChanged={onChanged}
          onError={setError}
        />
      </Section>

      <Section title={t('company.dossier.pastContracts.title', 'Past contracts')}>
        <RecordList
          empty={t('company.dossier.pastContracts.empty', 'No past contract recorded.')}
          records={dossier?.pastContracts ?? []}
          keyOf={(p: PastContract) => p.id}
          renderLabel={(p: PastContract) => p.description || p.buyerName}
          renderDetail={(p: PastContract) =>
            detailLine(
              p.buyerName,
              p.valueSet ? `${major(p.valueMinor)} ${p.currency}` : '',
              p.sharePctSet ? `${p.sharePct}%` : '',
            )
          }
          attributionOf={(p: PastContract) => p.attribution}
          onRemove={(p: PastContract) =>
            companyClient.removePastContract({ workspaceId, id: p.id }).then(onChanged)
          }
          onError={setError}
        />
        <PastContractAdd
          workspaceId={workspaceId}
          location={location}
          onChanged={onChanged}
          onError={setError}
        />
      </Section>

      <Section title={t('company.dossier.registrations.title', 'Registrations')}>
        <RecordList
          empty={t('company.dossier.registrations.empty', 'No registration recorded.')}
          records={dossier?.registrations ?? []}
          keyOf={(r: Registration) => r.id}
          renderLabel={(r: Registration) => r.kind}
          renderDetail={(r: Registration) => detailLine(r.authority, r.section, r.validUntil)}
          attributionOf={(r: Registration) => r.attribution}
          onRemove={(r: Registration) =>
            companyClient.removeRegistration({ workspaceId, id: r.id }).then(onChanged)
          }
          onError={setError}
        />
        <RegistrationAdd
          workspaceId={workspaceId}
          location={location}
          onChanged={onChanged}
          onError={setError}
        />
      </Section>
    </div>
  );
}

/**
 * Registry identity — what a DGUE Part II reads straight off.
 *
 * Identity is deliberately NOT captured just-in-time: no requirement kind maps to
 * it, it is never a blocking gap, and asking for a VAT number mid-decision is
 * exactly the friction the product refuses. It lives here and only here.
 */
function IdentitySection({
  workspaceId,
  dossier,
  location,
  onChanged,
  onError,
}: {
  workspaceId: string;
  dossier: CompanyDossier | null;
  location: AnalyticsLocation;
  onChanged: () => void;
  onError: (message: string | null) => void;
}) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const identity = dossier?.identity;

  // Local form state seeded from the server record — "backend-synced; local form
  // state only", the same shape `client-profile-form` uses.
  const [legalName, setLegalName] = useState(identity?.legalName ?? '');
  const [vatNumber, setVatNumber] = useState(identity?.vatNumber ?? '');
  const [fiscalCode, setFiscalCode] = useState(identity?.fiscalCode ?? '');
  const [legalForm, setLegalForm] = useState(identity?.legalForm ?? '');
  const [cciaaOffice, setCciaaOffice] = useState(identity?.cciaaOffice ?? '');
  const [cciaaNumber, setCciaaNumber] = useState(identity?.cciaaNumber ?? '');
  const [country, setCountry] = useState(identity?.country ?? '');
  const [nuts, setNuts] = useState(identity?.nuts ?? '');
  const [foundedYear, setFoundedYear] = useState(
    identity?.foundedYearSet ? String(identity.foundedYear) : '',
  );
  const [saving, setSaving] = useState(false);

  async function save() {
    onError(null);
    setSaving(true);
    try {
      const year = Number.parseInt(foundedYear.trim(), 10);
      await companyClient.updateCompanyIdentity({
        workspaceId,
        identity: {
          legalName,
          vatNumber,
          fiscalCode,
          legalForm,
          cciaaOffice,
          cciaaNumber,
          country: country.toUpperCase(),
          nuts: nuts.toUpperCase(),
          foundedYear: Number.isFinite(year) ? year : 0,
          foundedYearSet: Number.isFinite(year),
        },
        // Context only: the server derives provenance/statedBy/statedAt from how
        // the call arrived and ignores anything a client sends for them.
        attribution: { sourceNote: 'settings' },
      });
      capture('company_dossier_field_answered', {
        location,
        field_category: 'identity',
        prompted_by: 'settings',
      });
      onChanged();
    } catch (e: unknown) {
      onError(e instanceof Error ? e.message : t('company.dossier.error', 'Could not save.'));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Section title={t('company.dossier.title', 'Company dossier')}>
      <p className="text-sm text-ink-500">
        {t(
          'company.dossier.description',
          'What we hold about your company. Used to check whether you may be admitted to a gara at all.',
        )}
      </p>
      <div className="grid gap-3 sm:grid-cols-2">
        <TextInput
          label={t('company.dossier.identity.legalName', 'Legal name')}
          value={legalName}
          onChange={setLegalName}
        />
        <ChipRow
          label={t('company.dossier.identity.legalForm', 'Legal form')}
          options={[...LEGAL_FORMS]}
          value={legalForm}
          onChange={setLegalForm}
          translateKey="company.dossier.identity.legalForms"
        />
        <TextInput
          label={t('company.dossier.identity.vatNumber', 'VAT number')}
          value={vatNumber}
          onChange={setVatNumber}
        />
        {/* Two fields, not one: for an Italian company they usually coincide, and
            for a ditta individuale they do not. */}
        <TextInput
          label={t('company.dossier.identity.fiscalCode', 'Fiscal code')}
          value={fiscalCode}
          onChange={setFiscalCode}
        />
        <TextInput
          label={t('company.dossier.identity.cciaaOffice', 'CCIAA office')}
          value={cciaaOffice}
          onChange={setCciaaOffice}
        />
        <TextInput
          label={t('company.dossier.identity.cciaaNumber', 'REA number')}
          value={cciaaNumber}
          onChange={setCciaaNumber}
        />
        <TextInput
          label={t('company.dossier.identity.country', 'Country')}
          value={country}
          onChange={setCountry}
        />
        <TextInput
          label={t('company.dossier.identity.nuts', 'NUTS region')}
          value={nuts}
          onChange={setNuts}
        />
        <TextInput
          label={t('company.dossier.identity.foundedYear', 'Founded')}
          type="number"
          value={foundedYear}
          onChange={setFoundedYear}
        />
      </div>
      <Button isDisabled={saving} onPress={() => void save()} className={PRIMARY_BUTTON}>
        {saving ? t('company.dossier.saving', 'Saving…') : t('company.dossier.save', 'Save')}
      </Button>
    </Section>
  );
}

function SoaAdd({ workspaceId, location, onChanged, onError }: AddProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [category, setCategory] = useState('');
  const [classifica, setClassifica] = useState('');
  const [issuedBy, setIssuedBy] = useState('');
  const [validUntil, setValidUntil] = useState('');
  const [verifiedUntil, setVerifiedUntil] = useState('');
  const [saving, setSaving] = useState(false);

  async function add() {
    onError(null);
    setSaving(true);
    try {
      await companyClient.putSoaCategory({
        workspaceId,
        soa: {
          category,
          classifica,
          issuedBy,
          validUntil,
          verifiedUntil,
          attribution: { sourceNote: 'settings' },
        },
      });
      capture('company_dossier_field_answered', {
        location,
        field_category: 'soa',
        prompted_by: 'settings',
      });
      setCategory('');
      setClassifica('');
      setIssuedBy('');
      setValidUntil('');
      setVerifiedUntil('');
      onChanged();
    } catch (e: unknown) {
      onError(e instanceof Error ? e.message : t('company.dossier.error', 'Could not save.'));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-cream-200 p-3">
      <ChipRow
        label={t('company.dossier.soa.category', 'Category')}
        options={SOA_CATEGORIES}
        value={category}
        onChange={setCategory}
      />
      <ChipRow
        label={t('company.dossier.soa.classifica', 'Classifica')}
        options={[...SOA_CLASSIFICHE]}
        value={classifica}
        onChange={setClassifica}
      />
      <div className="grid gap-3 sm:grid-cols-3">
        <TextInput
          label={t('company.dossier.soa.issuedBy', 'Issued by')}
          value={issuedBy}
          onChange={setIssuedBy}
        />
        <TextInput
          label={t('company.dossier.soa.validUntil', 'Valid until')}
          type="date"
          value={validUntil}
          onChange={setValidUntil}
        />
        {/* The triennial verification lapses three years before validity and
            invalidates the attestation on its own. A form that asked only for
            validUntil would record a lapsed SOA as valid for two more years. */}
        <TextInput
          label={t('company.dossier.soa.verifiedUntil', 'Verified until')}
          hint={t(
            'company.dossier.soa.verifiedUntilHint',
            'The triennial verification date — it lapses before the attestation itself.',
          )}
          type="date"
          value={verifiedUntil}
          onChange={setVerifiedUntil}
        />
      </div>
      <Button isDisabled={saving} onPress={() => void add()} className={GHOST_BUTTON}>
        {t('company.dossier.soa.add', 'Add attestation')}
      </Button>
    </div>
  );
}

function CertificationAdd({ workspaceId, location, onChanged, onError }: AddProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [standard, setStandard] = useState('');
  const [standardRaw, setStandardRaw] = useState('');
  const [scope, setScope] = useState('');
  const [issuedBy, setIssuedBy] = useState('');
  const [validFrom, setValidFrom] = useState('');
  const [validUntil, setValidUntil] = useState('');
  const [saving, setSaving] = useState(false);

  async function add() {
    onError(null);
    setSaving(true);
    try {
      await companyClient.putCertification({
        workspaceId,
        certification: {
          standard,
          standardRaw,
          scope,
          issuedBy,
          validFrom,
          validUntil,
          attribution: { sourceNote: 'settings' },
        },
      });
      capture('company_dossier_field_answered', {
        location,
        field_category: 'certification',
        prompted_by: 'settings',
      });
      setStandard('');
      setStandardRaw('');
      setScope('');
      setIssuedBy('');
      setValidFrom('');
      setValidUntil('');
      onChanged();
    } catch (e: unknown) {
      onError(e instanceof Error ? e.message : t('company.dossier.error', 'Could not save.'));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-cream-200 p-3">
      <ChipRow
        label={t('company.dossier.certifications.standard', 'Standard')}
        options={[...CERTIFICATION_STANDARDS]}
        value={standard}
        onChange={setStandard}
        translateKey="company.dossier.certifications.standards"
      />
      {standard === 'other' && (
        <TextInput
          label={t('company.dossier.certifications.standardRaw', 'Published name')}
          value={standardRaw}
          onChange={setStandardRaw}
        />
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        {/* A certificate is scope-bounded: ISO 9001 for one activity does not
            cover another, and a dossier that drops the scope overclaims. */}
        <TextInput
          label={t('company.dossier.certifications.scope', 'Certified scope')}
          value={scope}
          onChange={setScope}
        />
        <TextInput
          label={t('company.dossier.certifications.issuedBy', 'Issued by')}
          value={issuedBy}
          onChange={setIssuedBy}
        />
        <TextInput
          label={t('company.dossier.certifications.validFrom', 'Valid from')}
          type="date"
          value={validFrom}
          onChange={setValidFrom}
        />
        <TextInput
          label={t('company.dossier.certifications.validUntil', 'Valid until')}
          type="date"
          value={validUntil}
          onChange={setValidUntil}
        />
      </div>
      <Button isDisabled={saving} onPress={() => void add()} className={GHOST_BUTTON}>
        {t('company.dossier.certifications.add', 'Add certification')}
      </Button>
    </div>
  );
}

function FinancialYearAdd({ workspaceId, location, onChanged, onError }: AddProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [year, setYear] = useState('');
  const [turnover, setTurnover] = useState('');
  const [specificTurnover, setSpecificTurnover] = useState('');
  const [currency, setCurrency] = useState('EUR');
  const [headcount, setHeadcount] = useState('');
  const [saving, setSaving] = useState(false);

  async function add() {
    onError(null);
    setSaving(true);
    try {
      const total = minor(turnover);
      const specific = minor(specificTurnover);
      const staff = Number.parseInt(headcount.trim(), 10);
      await companyClient.putFinancialYear({
        workspaceId,
        // Turnover and headcount share one record because Italian selection
        // criteria ask for both per esercizio.
        financialYear: {
          year: Number.parseInt(year.trim(), 10) || 0,
          turnoverMinor: total.value,
          turnoverSet: total.set,
          specificTurnoverMinor: specific.value,
          specificTurnoverSet: specific.set,
          currency,
          headcount: Number.isFinite(staff) ? staff : 0,
          headcountSet: Number.isFinite(staff),
          attribution: { sourceNote: 'settings' },
        },
      });
      capture('company_dossier_field_answered', {
        location,
        field_category: 'turnover',
        prompted_by: 'settings',
      });
      setYear('');
      setTurnover('');
      setSpecificTurnover('');
      setHeadcount('');
      onChanged();
    } catch (e: unknown) {
      onError(e instanceof Error ? e.message : t('company.dossier.error', 'Could not save.'));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="grid gap-3 rounded-xl border border-cream-200 p-3 sm:grid-cols-2">
      <TextInput
        label={t('company.dossier.financialYears.year', 'Year')}
        type="number"
        value={year}
        onChange={setYear}
      />
      <TextInput
        label={t('company.dossier.financialYears.currency', 'Currency')}
        value={currency}
        onChange={(v) => setCurrency(v.toUpperCase())}
      />
      <TextInput
        label={t('company.dossier.financialYears.turnover', 'Turnover')}
        type="number"
        value={turnover}
        onChange={setTurnover}
      />
      <TextInput
        label={t('company.dossier.financialYears.specificTurnover', 'Turnover in the sector')}
        type="number"
        value={specificTurnover}
        onChange={setSpecificTurnover}
      />
      <TextInput
        label={t('company.dossier.financialYears.headcount', 'Headcount')}
        type="number"
        value={headcount}
        onChange={setHeadcount}
      />
      <div className="flex items-end">
        <Button isDisabled={saving} onPress={() => void add()} className={GHOST_BUTTON}>
          {t('company.dossier.financialYears.add', 'Add year')}
        </Button>
      </div>
    </div>
  );
}

function PastContractAdd({ workspaceId, location, onChanged, onError }: AddProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [description, setDescription] = useState('');
  const [buyerName, setBuyerName] = useState('');
  const [cpv, setCpv] = useState('');
  const [value, setValue] = useState('');
  const [currency, setCurrency] = useState('EUR');
  const [startedOn, setStartedOn] = useState('');
  const [endedOn, setEndedOn] = useState('');
  const [role, setRole] = useState('');
  const [sharePct, setSharePct] = useState('');
  const [saving, setSaving] = useState(false);

  async function add() {
    onError(null);
    setSaving(true);
    try {
      const amount = minor(value);
      const share = Number.parseFloat(sharePct.trim().replace(',', '.'));
      await companyClient.putPastContract({
        workspaceId,
        pastContract: {
          description,
          buyerName,
          cpv,
          valueMinor: amount.value,
          valueSet: amount.set,
          currency,
          startedOn,
          endedOn,
          role,
          sharePct: Number.isFinite(share) ? share : 0,
          sharePctSet: Number.isFinite(share),
          attribution: { sourceNote: 'settings' },
        },
      });
      capture('company_dossier_field_answered', {
        location,
        field_category: 'past_contracts',
        prompted_by: 'settings',
      });
      setDescription('');
      setBuyerName('');
      setCpv('');
      setValue('');
      setStartedOn('');
      setEndedOn('');
      setRole('');
      setSharePct('');
      onChanged();
    } catch (e: unknown) {
      onError(e instanceof Error ? e.message : t('company.dossier.error', 'Could not save.'));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-cream-200 p-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <TextInput
          label={t('company.dossier.pastContracts.description', 'What the contract was')}
          value={description}
          onChange={setDescription}
        />
        <TextInput
          label={t('company.dossier.pastContracts.buyer', 'Buyer')}
          value={buyerName}
          onChange={setBuyerName}
        />
        <TextInput
          label={t('company.dossier.pastContracts.cpv', 'CPV')}
          value={cpv}
          onChange={setCpv}
        />
        <TextInput
          label={t('company.dossier.pastContracts.value', 'Value')}
          type="number"
          value={value}
          onChange={setValue}
        />
        <TextInput
          label={t('company.dossier.financialYears.currency', 'Currency')}
          value={currency}
          onChange={(v) => setCurrency(v.toUpperCase())}
        />
        <TextInput
          label={t('company.dossier.pastContracts.started', 'Start date')}
          type="date"
          value={startedOn}
          onChange={setStartedOn}
        />
        <TextInput
          label={t('company.dossier.pastContracts.ended', 'End date')}
          type="date"
          value={endedOn}
          onChange={setEndedOn}
        />
        {/* A 20% mandante on a €5M contract carries €1M of reference, not €5M. */}
        <TextInput
          label={t('company.dossier.pastContracts.sharePct', 'Your share (%)')}
          hint={t(
            'company.dossier.pastContracts.sharePctHint',
            'Only your own share counts toward a requirement.',
          )}
          type="number"
          value={sharePct}
          onChange={setSharePct}
        />
      </div>
      <ChipRow
        label={t('company.dossier.pastContracts.role', 'Role')}
        options={[...PAST_CONTRACT_ROLES]}
        value={role}
        onChange={setRole}
        translateKey="company.dossier.pastContracts.roles"
      />
      <Button isDisabled={saving} onPress={() => void add()} className={GHOST_BUTTON}>
        {t('company.dossier.pastContracts.add', 'Add contract')}
      </Button>
    </div>
  );
}

function RegistrationAdd({ workspaceId, location, onChanged, onError }: AddProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [kind, setKind] = useState('');
  const [authority, setAuthority] = useState('');
  const [identifier, setIdentifier] = useState('');
  const [section, setSection] = useState('');
  const [validFrom, setValidFrom] = useState('');
  const [validUntil, setValidUntil] = useState('');
  const [saving, setSaving] = useState(false);

  async function add() {
    onError(null);
    setSaving(true);
    try {
      await companyClient.putRegistration({
        workspaceId,
        registration: {
          kind,
          authority,
          identifier,
          section,
          validFrom,
          validUntil,
          attribution: { sourceNote: 'settings' },
        },
      });
      // The registration NUMBER is never sent to analytics — only that one was
      // recorded, and of which kind.
      capture('company_dossier_field_answered', {
        location,
        field_category: 'registration',
        prompted_by: 'settings',
      });
      setKind('');
      setAuthority('');
      setIdentifier('');
      setSection('');
      setValidFrom('');
      setValidUntil('');
      onChanged();
    } catch (e: unknown) {
      onError(e instanceof Error ? e.message : t('company.dossier.error', 'Could not save.'));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-cream-200 p-3">
      <ChipRow
        label={t('company.dossier.registrations.kind', 'Register')}
        options={[...REGISTRATION_KINDS]}
        value={kind}
        onChange={setKind}
        translateKey="company.dossier.registrations.kinds"
      />
      <div className="grid gap-3 sm:grid-cols-2">
        <TextInput
          label={t('company.dossier.registrations.authority', 'Authority')}
          value={authority}
          onChange={setAuthority}
        />
        <TextInput
          label={t('company.dossier.registrations.identifier', 'Registration number')}
          value={identifier}
          onChange={setIdentifier}
        />
        <TextInput
          label={t('company.dossier.registrations.section', 'Section')}
          value={section}
          onChange={setSection}
        />
        <TextInput
          label={t('company.dossier.registrations.validFrom', 'Valid from')}
          type="date"
          value={validFrom}
          onChange={setValidFrom}
        />
        <TextInput
          label={t('company.dossier.registrations.validUntil', 'Valid until')}
          type="date"
          value={validUntil}
          onChange={setValidUntil}
        />
      </div>
      <Button isDisabled={saving} onPress={() => void add()} className={GHOST_BUTTON}>
        {t('company.dossier.registrations.add', 'Add registration')}
      </Button>
    </div>
  );
}

type AddProps = {
  workspaceId: string;
  location: AnalyticsLocation;
  onChanged: () => void;
  onError: (message: string | null) => void;
};

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card>
      <h3 className="text-sm font-semibold text-ink-700">{title}</h3>
      <div className="mt-3 flex flex-col gap-3">{children}</div>
    </Card>
  );
}

function RecordList<T>({
  empty,
  records,
  keyOf,
  renderLabel,
  renderDetail,
  attributionOf,
  onRemove,
  onError,
}: {
  empty: string;
  records: T[];
  /**
   * The record's own identity. Not `record.id`: a FinancialYear has no id — the
   * year IS its key — so the caller says which field carries identity rather than
   * this component assuming every dossier record has the same one.
   */
  keyOf: (record: T) => string;
  renderLabel: (record: T) => string;
  renderDetail: (record: T) => string;
  attributionOf: (record: T) => { provenance?: string; statedAt?: string } | undefined;
  onRemove: (record: T) => Promise<unknown>;
  onError: (message: string | null) => void;
}) {
  const { t } = useTranslation();
  if (records.length === 0) return <p className="text-sm text-ink-400">{empty}</p>;
  return (
    <ul className="divide-y divide-cream-200">
      {records.map((record) => (
        <li key={keyOf(record)} className="flex items-start justify-between gap-3 py-2">
          <div className="min-w-0">
            <p className="text-sm text-ink-900">{renderLabel(record)}</p>
            {renderDetail(record) && <p className="text-xs text-ink-500">{renderDetail(record)}</p>}
            <ProvenanceMark
              attribution={attributionOf(record)}
              detail={attributionOf(record)?.statedAt}
              className="mt-1"
            />
          </div>
          <Button
            onPress={() => {
              onError(null);
              void onRemove(record).catch((e: unknown) =>
                onError(e instanceof Error ? e.message : 'Could not remove that.'),
              );
            }}
            className={QUIET_BUTTON}
          >
            {t('company.dossier.soa.remove', 'Remove')}
          </Button>
        </li>
      ))}
    </ul>
  );
}

function ChipRow({
  label,
  options,
  value,
  onChange,
  translateKey,
}: {
  label: string;
  options: string[];
  value: string;
  onChange: (value: string) => void;
  translateKey?: string;
}) {
  const { t } = useTranslation();
  return (
    <div>
      <p className="mb-2 text-sm font-medium text-ink-700">{label}</p>
      <div className="flex max-h-32 flex-wrap gap-2 overflow-y-auto">
        {options.map((option) => (
          <Button
            key={option}
            aria-pressed={value === option}
            onPress={() => onChange(value === option ? '' : option)}
            className={cn(CHIP_BASE, value === option ? CHIP_ON : CHIP_OFF)}
          >
            {translateKey ? t(`${translateKey}.${option}`, option) : option}
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

/** Joins the non-empty parts of a detail line — an empty segment prints nothing. */
function detailLine(...parts: string[]): string {
  return parts.filter((p) => p.trim().length > 0).join(' · ');
}

/** Minor units back to a human amount for display. */
function major(amountMinor: bigint): string {
  return (Number(amountMinor) / 100).toLocaleString();
}

/** A blank amount means "not asserted", never zero. */
function minor(amount: string): { value: bigint; set: boolean } {
  const trimmed = amount.trim();
  if (!trimmed) return { value: 0n, set: false };
  const parsed = Number.parseFloat(trimmed.replace(',', '.'));
  if (!Number.isFinite(parsed)) return { value: 0n, set: false };
  return { value: BigInt(Math.round(parsed * 100)), set: true };
}

const PRIMARY_BUTTON =
  'inline-flex h-10 w-fit items-center justify-center rounded-xl bg-brand-600 px-4 text-sm font-semibold text-white outline-none ' +
  'transition-colors duration-150 data-[hovered]:bg-brand-700 data-[pressed]:bg-brand-800 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2';

const GHOST_BUTTON =
  'inline-flex h-10 w-fit items-center justify-center rounded-xl border border-cream-300 bg-white px-4 text-sm font-semibold text-ink-800 outline-none ' +
  'transition-colors duration-150 data-[hovered]:border-cream-400 data-[pressed]:bg-cream-100 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

const QUIET_BUTTON =
  'inline-flex h-10 shrink-0 items-center justify-center rounded-xl px-3 text-xs font-semibold text-ink-500 outline-none ' +
  'transition-colors duration-150 data-[hovered]:bg-cream-200 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

const CHIP_BASE =
  'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors outline-none ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';
const CHIP_ON = 'border-brand-600 bg-brand-100 text-brand-700';
const CHIP_OFF = 'border-cream-300 bg-white text-ink-600 data-[hovered]:border-cream-400';

const TEXT_INPUT =
  'h-10 w-full rounded-xl border border-cream-300 bg-white px-3 text-sm text-ink-900 outline-none ' +
  'transition-colors duration-150 placeholder:text-ink-300 focus:border-brand-600 focus:ring-2 focus:ring-brand-600/25';
