import { describe, expect, it } from 'vitest';
import { tenderTitle } from './title';

describe('tenderTitle', () => {
  it('strips the leading country name from a real notice title', () => {
    const raw =
      'Italia – Apparecchi per angiografia – Affidamento della fornitura ' +
      '"chiavi in mano" di n. 1 angiografo biplano';
    expect(tenderTitle(raw, 'ITA')).toBe(
      'Apparecchi per angiografia – Affidamento della fornitura ' +
        '"chiavi in mano" di n. 1 angiografo biplano',
    );
  });

  it('matches the country name in any EU language, not just the local one', () => {
    expect(tenderTitle('Italy – Angiography equipment', 'ITA')).toBe('Angiography equipment');
    expect(tenderTitle('France – Rénovation énergétique', 'FRA')).toBe('Rénovation énergétique');
    expect(tenderTitle('Deutschland – IT-Beratung', 'DEU')).toBe('IT-Beratung');
  });

  it('handles a colon separator as well as a dash', () => {
    expect(tenderTitle('Italia: Servizi di pulizia', 'ITA')).toBe('Servizi di pulizia');
  });

  it('keeps dashes that belong to the title, only dropping the country', () => {
    // The remaining "category – object" dash must survive.
    expect(tenderTitle('Portugal – Limpeza – Recolha de resíduos', 'PRT')).toBe(
      'Limpeza – Recolha de resíduos',
    );
  });

  it('leaves the title untouched when the lead segment is not the country', () => {
    expect(tenderTitle('Framework agreement – supply of vehicles', 'ITA')).toBe(
      'Framework agreement – supply of vehicles',
    );
    expect(tenderTitle('IT-Sicherheitsberatung und Penetrationstests', 'DEU')).toBe(
      'IT-Sicherheitsberatung und Penetrationstests',
    );
  });

  it('leaves the title untouched when the country is unknown', () => {
    expect(tenderTitle('Italia – Apparecchi', 'ZZZ')).toBe('Italia – Apparecchi');
  });

  it('returns a prefix-less title unchanged', () => {
    expect(tenderTitle('Supply of road maintenance services', 'PT')).toBe(
      'Supply of road maintenance services',
    );
  });
});
