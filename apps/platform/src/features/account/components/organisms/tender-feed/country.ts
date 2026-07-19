import AT from 'country-flag-icons/react/3x2/AT';
import BE from 'country-flag-icons/react/3x2/BE';
import BG from 'country-flag-icons/react/3x2/BG';
import CH from 'country-flag-icons/react/3x2/CH';
import CY from 'country-flag-icons/react/3x2/CY';
import CZ from 'country-flag-icons/react/3x2/CZ';
import DE from 'country-flag-icons/react/3x2/DE';
import DK from 'country-flag-icons/react/3x2/DK';
import EE from 'country-flag-icons/react/3x2/EE';
import ES from 'country-flag-icons/react/3x2/ES';
import EU from 'country-flag-icons/react/3x2/EU';
import FI from 'country-flag-icons/react/3x2/FI';
import FR from 'country-flag-icons/react/3x2/FR';
import GB from 'country-flag-icons/react/3x2/GB';
import GR from 'country-flag-icons/react/3x2/GR';
import HR from 'country-flag-icons/react/3x2/HR';
import HU from 'country-flag-icons/react/3x2/HU';
import IE from 'country-flag-icons/react/3x2/IE';
import IS from 'country-flag-icons/react/3x2/IS';
import IT from 'country-flag-icons/react/3x2/IT';
import LI from 'country-flag-icons/react/3x2/LI';
import LT from 'country-flag-icons/react/3x2/LT';
import LU from 'country-flag-icons/react/3x2/LU';
import LV from 'country-flag-icons/react/3x2/LV';
import MT from 'country-flag-icons/react/3x2/MT';
import NL from 'country-flag-icons/react/3x2/NL';
import NO from 'country-flag-icons/react/3x2/NO';
import PL from 'country-flag-icons/react/3x2/PL';
import PT from 'country-flag-icons/react/3x2/PT';
import RO from 'country-flag-icons/react/3x2/RO';
import SE from 'country-flag-icons/react/3x2/SE';
import SI from 'country-flag-icons/react/3x2/SI';
import SK from 'country-flag-icons/react/3x2/SK';

/** A `country-flag-icons` SVG flag component (accepts standard SVG/HTML props). */
export type FlagComponent = typeof AT;

/**
 * ISO 3166-1 alpha-2 → flag component. Covers the EU-27 plus the EEA/adjacent
 * states that also appear on TED (CH, GB, IS, LI, NO) and the EU institutions
 * flag — the realistic origins of a tender notice. Anything else resolves to
 * null and the card falls back to the raw country code.
 */
const FLAGS: Record<string, FlagComponent> = {
  AT,
  BE,
  BG,
  CH,
  CY,
  CZ,
  DE,
  DK,
  EE,
  ES,
  EU,
  FI,
  FR,
  GB,
  GR,
  HR,
  HU,
  IE,
  IS,
  IT,
  LI,
  LT,
  LU,
  LV,
  MT,
  NL,
  NO,
  PL,
  PT,
  RO,
  SE,
  SI,
  SK,
};

/**
 * ISO 3166-1 alpha-3 → alpha-2 for the countries we carry a flag for. Tender
 * data and `ClientProfile.Countries` are both alpha-2 in storage — TED/eforms'
 * source data is alpha-3, but the ingestion pipeline converts it to alpha-2
 * before storage. This map exists for defensive display-side resolution
 * (`countryAlpha2`/`countryFlag`/`countryName` accept either form), not as the
 * wire format — see `TENDER_COUNTRY_CODES` below for that.
 */
const ALPHA3_TO_ALPHA2: Record<string, string> = {
  AUT: 'AT',
  BEL: 'BE',
  BGR: 'BG',
  CHE: 'CH',
  CYP: 'CY',
  CZE: 'CZ',
  DEU: 'DE',
  DNK: 'DK',
  EST: 'EE',
  ESP: 'ES',
  FIN: 'FI',
  FRA: 'FR',
  GBR: 'GB',
  GRC: 'GR',
  HRV: 'HR',
  HUN: 'HU',
  IRL: 'IE',
  ISL: 'IS',
  ITA: 'IT',
  LIE: 'LI',
  LTU: 'LT',
  LUX: 'LU',
  LVA: 'LV',
  MLT: 'MT',
  NLD: 'NL',
  NOR: 'NO',
  POL: 'PL',
  PRT: 'PT',
  ROU: 'RO',
  SWE: 'SE',
  SVN: 'SI',
  SVK: 'SK',
};

/**
 * EU-internal code quirks: Eurostat/TED use "EL" for Greece and "UK" for the
 * United Kingdom instead of the ISO alpha-2 codes, and "EUU" for the Union.
 */
const ALIASES: Record<string, string> = { EL: 'GR', UK: 'GB', EUU: 'EU' };

/**
 * The alpha-2 country codes we can name and flag — the EU-27 plus the EEA/adjacent
 * states that also appear on TED — offered as the explore "country" filter and the
 * client-profile country picker. Both consumers send these values verbatim over the
 * wire (the search filter's `country`, `ClientProfile.Countries`), and the backend
 * validates/stores alpha-2 only (`clientprofile.go`'s `countryRe`,
 * `ingested_tenders.country`), so this must stay alpha-2 — not the alpha-3 keys of
 * `ALPHA3_TO_ALPHA2` above. Sort by localised name at the call site with `countryName`.
 */
export const TENDER_COUNTRY_CODES: readonly string[] = Object.values(ALPHA3_TO_ALPHA2);

/**
 * Resolves a raw tender country code (alpha-3, alpha-2, or an EU quirk code) to
 * an ISO 3166-1 alpha-2 region code, or null when it isn't one we recognise.
 */
export function countryAlpha2(code: string): string | null {
  const c = code.trim().toUpperCase();
  if (!c) return null;
  const resolved = ALPHA3_TO_ALPHA2[c] ?? ALIASES[c] ?? (c.length === 2 ? c : null);
  return resolved && resolved in FLAGS ? resolved : null;
}

/** The flag component for a tender country code, or null to fall back to text. */
export function countryFlag(code: string): FlagComponent | null {
  const alpha2 = countryAlpha2(code);
  return alpha2 ? (FLAGS[alpha2] ?? null) : null;
}

/**
 * The localised country name for the flag's accessible label and tooltip
 * (e.g. "Italy" / "Italia" per locale). Falls back to the alpha-2 code, then
 * the raw input, when Intl can't name the region (e.g. the EU flag).
 */
export function countryName(code: string, locale: string): string {
  const alpha2 = countryAlpha2(code);
  if (alpha2) {
    try {
      const name = new Intl.DisplayNames([locale], { type: 'region' }).of(alpha2);
      if (name && name !== alpha2) return name;
    } catch {
      // Intl unavailable or region not nameable — fall through to the code.
    }
    return alpha2;
  }
  return code.trim().toUpperCase();
}
