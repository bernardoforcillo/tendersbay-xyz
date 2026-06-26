import type { EuCountry } from './flags';

/**
 * National public e-procurement portal per EU country — the platform where each
 * country publishes its public tenders. Names are intentionally short and
 * recognizable; edit freely as coverage details firm up.
 */
export const PORTALS: Record<EuCountry, string> = {
  AT: 'BBG (Bundesbeschaffung)',
  BE: 'e-Procurement',
  BG: 'CAIS EOP',
  CY: 'eProcurement Cyprus',
  CZ: 'NEN',
  DE: 'e-Vergabe (bund.de)',
  DK: 'Udbud.dk',
  EE: 'Riigihangete register',
  ES: 'PLACSP',
  FI: 'HILMA',
  FR: 'PLACE',
  GR: 'Promitheus (ΕΣΗΔΗΣ)',
  HR: 'EOJN',
  HU: 'EKR',
  IE: 'eTenders',
  IT: 'Acquisti in Rete (MEPA)',
  LT: 'CVP IS',
  LU: 'Portail des Marchés Publics',
  LV: 'EIS',
  MT: 'ePPS',
  NL: 'TenderNed',
  PL: 'Platforma e-Zamówienia',
  PT: 'Portal BASE',
  RO: 'SEAP (e-licitație)',
  SE: 'Upphandlingsmyndigheten',
  SI: 'e-JN',
  SK: 'EVO (ÚVO)',
};
