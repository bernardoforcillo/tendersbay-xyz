import type { Tender } from '~/features/landing/components/atoms';

// Placeholder examples (each in one European language) until real tenders are wired in.
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
];
