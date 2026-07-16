import type { Tender } from '~/features/landing/components/atoms';

// Illustrative sample tenders — NOT live data. Buyers are named in their own
// country's language and objects are plausible calls spread across EU markets
// and sectors (IT, energy, construction, health, transport, facilities,
// education). Values and deadlines are indicative only. Swapped for real
// tenders in phase 2 (see `fetchSampleTenders`).
export const SAMPLE_TENDERS: Tender[] = [
  {
    id: 'lis',
    entity: 'Câmara de Lisboa',
    object: 'Fornecimento de serviços de TI',
    value: '€ 240.000',
    deadlineDays: 12,
    scoutCount: 12,
  },
  {
    id: 'lyo',
    entity: 'Ville de Lyon',
    object: 'Rénovation énergétique',
    value: '€ 1.200.000',
    deadlineDays: 18,
    scoutCount: 31,
  },
  {
    id: 'muc',
    entity: 'Stadt München',
    object: 'IT-Sicherheitsberatung',
    value: '€ 350.000',
    deadlineDays: 9,
    scoutCount: 19,
  },
  {
    id: 'sev',
    entity: 'Ayuntamiento de Sevilla',
    object: 'Servicios de limpieza',
    value: '€ 480.000',
    deadlineDays: 21,
    scoutCount: 24,
  },
  {
    id: 'mil',
    entity: 'Comune di Milano',
    object: 'Manutenzione della flotta di autobus',
    value: '€ 3.400.000',
    deadlineDays: 16,
    scoutCount: 27,
  },
  {
    id: 'ams',
    entity: 'Gemeente Amsterdam',
    object: 'Renovatie van bruggen',
    value: '€ 2.800.000',
    deadlineDays: 20,
    scoutCount: 15,
  },
  {
    id: 'waw',
    entity: 'Szpital Wojewódzki w Warszawie',
    object: 'Dostawa sprzętu medycznego',
    value: '€ 540.000',
    deadlineDays: 11,
    scoutCount: 18,
  },
  {
    id: 'sto',
    entity: 'Stockholms stad',
    object: 'Läromedel för grundskolor',
    value: '€ 410.000',
    deadlineDays: 15,
    scoutCount: 9,
  },
  {
    id: 'vie',
    entity: 'Stadt Wien',
    object: 'Photovoltaik-Anlagen',
    value: '€ 1.750.000',
    deadlineDays: 19,
    scoutCount: 21,
  },
  {
    id: 'bru',
    entity: 'Ville de Bruxelles',
    object: 'Entretien des espaces verts',
    value: '€ 390.000',
    deadlineDays: 13,
    scoutCount: 11,
  },
  {
    id: 'hel',
    entity: 'Helsingin kaupunki',
    object: 'Koulun peruskorjaus',
    value: '€ 2.100.000',
    deadlineDays: 22,
    scoutCount: 14,
  },
  {
    id: 'dub',
    entity: 'Dublin City Council',
    object: 'Cloud infrastructure services',
    value: '€ 780.000',
    deadlineDays: 10,
    scoutCount: 20,
  },
  {
    id: 'cph',
    entity: 'Københavns Kommune',
    object: 'Elektriske bybusser',
    value: '€ 4.600.000',
    deadlineDays: 17,
    scoutCount: 33,
  },
  {
    id: 'pra',
    entity: 'Fakultní nemocnice v Praze',
    object: 'Dodávka zdravotnického vybavení',
    value: '€ 660.000',
    deadlineDays: 8,
    scoutCount: 16,
  },
  {
    id: 'ath',
    entity: 'Δήμος Αθηναίων',
    object: 'Εκπαιδευτικός εξοπλισμός σχολείων',
    value: '€ 520.000',
    deadlineDays: 14,
    scoutCount: 12,
  },
];

/** Non-mutating Fisher–Yates shuffle. */
function shuffle<T>(items: readonly T[]): T[] {
  const copy = [...items];
  for (let i = copy.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [copy[i], copy[j]] = [copy[j] as T, copy[i] as T];
  }
  return copy;
}

/** Clamp a requested count into `[0, pool size]` so the deck never over-reads. */
function clampCount(count: number): number {
  return Math.max(0, Math.min(count, SAMPLE_TENDERS.length));
}

/**
 * A deterministic first-`count` slice of the pool. Used as the hero's
 * synchronous initial value so the deck paints on the first frame with no empty
 * state and no LCP regression, before the async loader swaps in a random set.
 */
export function initialSampleTenders(count = 3): Tender[] {
  return SAMPLE_TENDERS.slice(0, clampCount(count));
}

/**
 * Async seam for the hero deck: resolves a randomised subset of the curated
 * pool as a Promise, so the hero awaits it exactly like a real fetch. These are
 * illustrative samples only — never surface them as live / real-time results.
 * `count` is clamped to the pool size so an over-large request cannot throw.
 *
 * // PHASE 2: replace body with tenderClient.searchTenders({ query: '', limit: count });
 * // map results to `Tender`, and fall back to the shuffled pool on error/empty.
 */
export function fetchSampleTenders(count = 3): Promise<Tender[]> {
  return Promise.resolve(shuffle(SAMPLE_TENDERS).slice(0, clampCount(count)));
}
