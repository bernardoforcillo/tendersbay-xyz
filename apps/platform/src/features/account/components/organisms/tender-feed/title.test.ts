import { describe, expect, it } from 'vitest';
import { tenderTitle } from './title';

describe('tenderTitle', () => {
  it('splits a real notice into object title and category subtitle', () => {
    const raw =
      'Italia – Apparecchi per angiografia – Affidamento della fornitura ' +
      '"chiavi in mano" di n. 1 angiografo biplano';
    expect(tenderTitle(raw, 'ITA')).toEqual({
      title: 'Affidamento della fornitura "chiavi in mano" di n. 1 angiografo biplano',
      category: 'Apparecchi per angiografia',
    });
  });

  it('matches the country name in any EU language, not just the local one', () => {
    expect(tenderTitle('Italy – Equipment – Angiography supply', 'ITA')).toEqual({
      title: 'Angiography supply',
      category: 'Equipment',
    });
    expect(tenderTitle('Deutschland – IT – IT-Sicherheitsberatung', 'DEU')).toEqual({
      title: 'IT-Sicherheitsberatung',
      category: 'IT',
    });
  });

  it('keeps further dashes with the object, only pulling the first category out', () => {
    expect(tenderTitle('Portugal – Limpeza – Recolha – e transporte de resíduos', 'PRT')).toEqual({
      title: 'Recolha – e transporte de resíduos',
      category: 'Limpeza',
    });
  });

  it('returns no category when the notice has only a country and an object', () => {
    expect(tenderTitle('Italia – Servizi di pulizia', 'ITA')).toEqual({
      title: 'Servizi di pulizia',
      category: null,
    });
  });

  it('accepts a colon after the country as well as a dash', () => {
    expect(tenderTitle('Italia: Apparecchi – Fornitura', 'ITA')).toEqual({
      title: 'Fornitura',
      category: 'Apparecchi',
    });
  });

  it('leaves the title whole when the lead segment is not the country', () => {
    expect(tenderTitle('Framework agreement – supply of vehicles', 'ITA')).toEqual({
      title: 'Framework agreement – supply of vehicles',
      category: null,
    });
    expect(tenderTitle('IT-Sicherheitsberatung und Penetrationstests', 'DEU')).toEqual({
      title: 'IT-Sicherheitsberatung und Penetrationstests',
      category: null,
    });
  });

  it('leaves the title whole when the country is unknown', () => {
    expect(tenderTitle('Italia – Apparecchi – Fornitura', 'ZZZ')).toEqual({
      title: 'Italia – Apparecchi – Fornitura',
      category: null,
    });
  });

  it('returns a prefix-less title unchanged with no category', () => {
    expect(tenderTitle('Supply of road maintenance services', 'PT')).toEqual({
      title: 'Supply of road maintenance services',
      category: null,
    });
  });
});
