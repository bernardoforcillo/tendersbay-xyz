import { Banner, Button, cn } from '@tendersbay/components/core';
import type { ClientProfile } from '@tendersbay/proto/workspace/v1/workspace_pb';
import { usePostHog } from 'posthog-js/react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { workspaceClient } from '~/lib/api/client';
import { CPV_SECTORS, countryName, TENDER_COUNTRY_CODES } from '../tender-feed';
import { band } from './band';
import { PROCEDURE_TYPES, type ProcedureType } from './procedure-type';
import { formatRegions, parseRegions } from './region';

export type ClientProfileFormProps = {
  workspaceId: string;
  initial?: ClientProfile;
  onSaved: (profile: ClientProfile) => void;
  /**
   * Analytics `location` tag on the `client_profile_completed` event — which
   * surface the form was submitted from (Explore, first-run capture, Settings).
   * Defaults to the form's original Explore surface so existing callers are
   * unaffected.
   */
  location?: string;
};

const TOGGLE_BASE =
  'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors outline-none ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';
const TOGGLE_ON = 'border-brand-600 bg-brand-100 text-brand-700';
const TOGGLE_OFF = 'border-cream-300 bg-white text-ink-600 hover:border-cream-400';

const TEXT_INPUT =
  'h-10 w-full rounded-xl border border-cream-300 bg-white px-3 text-sm text-ink-900 outline-none ' +
  'transition-colors duration-150 placeholder:text-ink-300 focus:border-brand-600 focus:ring-2 focus:ring-brand-600/25';

/**
 * English fallbacks for the procedure-type chip labels — real 24-locale copy
 * lands in Task 17. Keyed by the backend's exact literal (clientprofile.go's
 * validProcedureTypes), so a chip's i18n key is `explore.clientProfile.procedureTypes.<value>`.
 */
const PROCEDURE_TYPE_LABELS: Record<ProcedureType, string> = {
  open: 'Open procedure',
  restricted: 'Restricted procedure',
  negotiated: 'Negotiated procedure',
  competitive_dialogue: 'Competitive dialogue',
  innovation_partnership: 'Innovation partnership',
  other: 'Other',
};

/**
 * The per-workspace client bid profile: sectors, countries, NUTS regions, procedure
 * types, a value band, and free-text notes — everything RecommendTendersForClient
 * matches against. Sectors/countries/procedure-types are small closed sets, so they
 * render as toggle-button chips; regions are a large hierarchical taxonomy with no
 * enumerable list in this frontend, so that control is a free-text, comma-separated
 * input instead (parsed client-side with a light format nudge — the backend's
 * `ErrInvalidRegion` is the source of truth and surfaces through the error Banner
 * below like any other validation failure). The kit has no chip/textarea primitive,
 * so those render as native elements styled to the kit's tone system rather than
 * extending the shared kit for one feature's form.
 */
export function ClientProfileForm({
  workspaceId,
  initial,
  onSaved,
  location = 'explore_profile_form',
}: ClientProfileFormProps) {
  const { t, i18n } = useTranslation();
  const posthog = usePostHog();

  const [sectors, setSectors] = useState<string[]>(initial?.sectors ?? []);
  const [countries, setCountries] = useState<string[]>(initial?.countries ?? []);
  const [regionsText, setRegionsText] = useState(formatRegions(initial?.regions ?? []));
  const [procedureTypes, setProcedureTypes] = useState<string[]>(initial?.procedureTypes ?? []);
  const [valueMin, setValueMin] = useState(initial?.valueMinSet ? String(initial.valueMin) : '');
  const [valueMax, setValueMax] = useState(initial?.valueMaxSet ? String(initial.valueMax) : '');
  const [notes, setNotes] = useState(initial?.notes ?? '');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  function toggle(list: string[], setList: (v: string[]) => void, value: string) {
    setList(list.includes(value) ? list.filter((v) => v !== value) : [...list, value]);
  }

  async function handleSubmit() {
    setError(null);
    const min = valueMin.trim() ? BigInt(valueMin.trim()) : null;
    const max = valueMax.trim() ? BigInt(valueMax.trim()) : null;
    if (min !== null && max !== null && min > max) {
      setError(
        t('explore.clientProfile.invalidBand', {
          defaultValue: 'Minimum value must not exceed maximum value.',
        }),
      );
      return;
    }

    const regions = parseRegions(regionsText);

    setSaving(true);
    try {
      const res = await workspaceClient.updateClientProfile({
        workspaceId,
        sectors,
        countries,
        regions,
        procedureTypes,
        valueMin: min ?? 0n,
        valueMinSet: min !== null,
        valueMax: max ?? 0n,
        valueMaxSet: max !== null,
        notes,
      });
      // Consent-safe props only: counts are bucketed via band(), never sent raw, and
      // the free-text region values themselves are never sent to analytics — only
      // how many the advisor configured.
      posthog?.capture('client_profile_completed', {
        location,
        sector_count: band(sectors.length),
        country_count: band(countries.length),
        region_count: band(regions.length),
        has_value_band: min !== null || max !== null,
        has_procedure_filter: procedureTypes.length > 0,
      });
      onSaved(res.profile as ClientProfile);
    } catch (e: unknown) {
      setError(
        e instanceof Error
          ? e.message
          : t('explore.clientProfile.error', {
              defaultValue: 'Could not save the profile — try again.',
            }),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <p className="mb-2 text-sm font-medium text-ink-700">
          {t('explore.clientProfile.sectors', { defaultValue: 'Sectors' })}
        </p>
        <div className="flex flex-wrap gap-2">
          {CPV_SECTORS.map(({ key, prefix }) => (
            <button
              key={key}
              type="button"
              aria-pressed={sectors.includes(prefix)}
              onClick={() => toggle(sectors, setSectors, prefix)}
              className={cn(TOGGLE_BASE, sectors.includes(prefix) ? TOGGLE_ON : TOGGLE_OFF)}
            >
              {t(`tenders.filters.sectors.${key}`)}
            </button>
          ))}
        </div>
      </div>

      <div>
        <p className="mb-2 text-sm font-medium text-ink-700">
          {t('explore.clientProfile.countries', { defaultValue: 'Countries' })}
        </p>
        <div className="flex max-h-40 flex-wrap gap-2 overflow-y-auto">
          {TENDER_COUNTRY_CODES.map((code) => (
            <button
              key={code}
              type="button"
              aria-pressed={countries.includes(code)}
              onClick={() => toggle(countries, setCountries, code)}
              className={cn(TOGGLE_BASE, countries.includes(code) ? TOGGLE_ON : TOGGLE_OFF)}
            >
              {countryName(code, i18n.language)}
            </button>
          ))}
        </div>
      </div>

      <div>
        <p className="mb-2 text-sm font-medium text-ink-700">
          {t('explore.clientProfile.procedureTypesLabel', { defaultValue: 'Procedure types' })}
        </p>
        <div className="flex flex-wrap gap-2">
          {PROCEDURE_TYPES.map((type) => (
            <button
              key={type}
              type="button"
              aria-pressed={procedureTypes.includes(type)}
              onClick={() => toggle(procedureTypes, setProcedureTypes, type)}
              className={cn(TOGGLE_BASE, procedureTypes.includes(type) ? TOGGLE_ON : TOGGLE_OFF)}
            >
              {t(`explore.clientProfile.procedureTypes.${type}`, {
                defaultValue: PROCEDURE_TYPE_LABELS[type],
              })}
            </button>
          ))}
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <label className="flex flex-col gap-1.5 text-sm font-medium text-ink-700">
          {t('explore.clientProfile.regions', { defaultValue: 'Regions (NUTS)' })}
          <input
            type="text"
            className={TEXT_INPUT}
            value={regionsText}
            onChange={(e) => setRegionsText(e.target.value)}
            placeholder={t('explore.clientProfile.regionsPlaceholder', {
              defaultValue: 'e.g. ITC, DE3',
            })}
          />
        </label>
        <p className="text-xs text-ink-400">
          {t('explore.clientProfile.regionsHint', {
            defaultValue: 'Comma-separated NUTS prefixes. Leave blank to match any region.',
          })}
        </p>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <label className="flex flex-col gap-1.5 text-sm font-medium text-ink-700">
          {t('explore.clientProfile.valueMin', { defaultValue: 'Minimum value' })}
          <input
            type="number"
            className={TEXT_INPUT}
            value={valueMin}
            onChange={(e) => setValueMin(e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-medium text-ink-700">
          {t('explore.clientProfile.valueMax', { defaultValue: 'Maximum value' })}
          <input
            type="number"
            className={TEXT_INPUT}
            value={valueMax}
            onChange={(e) => setValueMax(e.target.value)}
          />
        </label>
      </div>

      <label className="flex flex-col gap-1.5 text-sm font-medium text-ink-700">
        {t('explore.clientProfile.notes', { defaultValue: 'Notes' })}
        <textarea
          className={cn(TEXT_INPUT, 'h-24 resize-none py-2')}
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          placeholder={t('explore.clientProfile.notesPlaceholder', {
            defaultValue: 'What does this client typically bid on?',
          })}
        />
      </label>

      {error && <Banner tone="error">{error}</Banner>}

      <Button onPress={handleSubmit} isDisabled={saving}>
        {t('explore.clientProfile.save', { defaultValue: 'Save profile' })}
      </Button>
    </div>
  );
}
