import AT from 'country-flag-icons/react/3x2/AT';
import BE from 'country-flag-icons/react/3x2/BE';
import BG from 'country-flag-icons/react/3x2/BG';
import CY from 'country-flag-icons/react/3x2/CY';
import CZ from 'country-flag-icons/react/3x2/CZ';
import DE from 'country-flag-icons/react/3x2/DE';
import DK from 'country-flag-icons/react/3x2/DK';
import EE from 'country-flag-icons/react/3x2/EE';
import ES from 'country-flag-icons/react/3x2/ES';
import FI from 'country-flag-icons/react/3x2/FI';
import FR from 'country-flag-icons/react/3x2/FR';
import GR from 'country-flag-icons/react/3x2/GR';
import HR from 'country-flag-icons/react/3x2/HR';
import HU from 'country-flag-icons/react/3x2/HU';
import IE from 'country-flag-icons/react/3x2/IE';
import IT from 'country-flag-icons/react/3x2/IT';
import LT from 'country-flag-icons/react/3x2/LT';
import LU from 'country-flag-icons/react/3x2/LU';
import LV from 'country-flag-icons/react/3x2/LV';
import MT from 'country-flag-icons/react/3x2/MT';
import NL from 'country-flag-icons/react/3x2/NL';
import PL from 'country-flag-icons/react/3x2/PL';
import PT from 'country-flag-icons/react/3x2/PT';
import RO from 'country-flag-icons/react/3x2/RO';
import SE from 'country-flag-icons/react/3x2/SE';
import SI from 'country-flag-icons/react/3x2/SI';
import SK from 'country-flag-icons/react/3x2/SK';

/** A `country-flag-icons` SVG flag component (accepts standard SVG/HTML props). */
export type FlagComponent = typeof AT;

/** 27 EU member states, ISO 3166-1 alpha-2, in alphabetical-by-code display order. */
export const EU_COUNTRIES = [
  'AT',
  'BE',
  'BG',
  'CY',
  'CZ',
  'DE',
  'DK',
  'EE',
  'ES',
  'FI',
  'FR',
  'GR',
  'HR',
  'HU',
  'IE',
  'IT',
  'LT',
  'LU',
  'LV',
  'MT',
  'NL',
  'PL',
  'PT',
  'RO',
  'SE',
  'SI',
  'SK',
] as const;

export type EuCountry = (typeof EU_COUNTRIES)[number];

/** ISO code → flag component. Keys mirror EU_COUNTRIES exactly. */
export const FLAGS: Record<EuCountry, FlagComponent> = {
  AT,
  BE,
  BG,
  CY,
  CZ,
  DE,
  DK,
  EE,
  ES,
  FI,
  FR,
  GR,
  HR,
  HU,
  IE,
  IT,
  LT,
  LU,
  LV,
  MT,
  NL,
  PL,
  PT,
  RO,
  SE,
  SI,
  SK,
};
