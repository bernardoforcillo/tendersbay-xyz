import { describe, expect, it } from 'vitest';
import { fitReasonFragments, fitTierPillClassName, fitTierPillTone } from './fit';

describe('fitTierPillTone', () => {
  it('returns match for strong', () => {
    expect(fitTierPillTone('strong')).toBe('match');
  });
  it('returns neutral for possible', () => {
    expect(fitTierPillTone('possible')).toBe('neutral');
  });
  it('returns neutral for long_shot', () => {
    expect(fitTierPillTone('long_shot')).toBe('neutral');
  });
});

describe('fitTierPillClassName', () => {
  it('applies grayscale only to long_shot (no gray token, per frontend rules)', () => {
    expect(fitTierPillClassName('long_shot')).toBe('grayscale');
    expect(fitTierPillClassName('strong')).toBeUndefined();
    expect(fitTierPillClassName('possible')).toBeUndefined();
  });
});

describe('fitReasonFragments', () => {
  it('returns no fragments when nothing matches', () => {
    const got = fitReasonFragments({
      sectorMatch: false,
      countryMatch: false,
      valueFit: 'unknown',
      deadlineDays: 0,
      hasDeadline: false,
      regionMatch: false,
      procedureMatch: false,
    });
    expect(got).toEqual([]);
  });

  it('includes every matching signal, in a stable order', () => {
    const got = fitReasonFragments({
      sectorMatch: true,
      countryMatch: true,
      valueFit: 'in_band',
      deadlineDays: 12,
      hasDeadline: true,
      regionMatch: false,
      procedureMatch: false,
    });
    expect(got).toEqual([
      { key: 'tenders.fit.reasonSector' },
      { key: 'tenders.fit.reasonCountry' },
      { key: 'tenders.fit.reasonValueInBand' },
      { key: 'tenders.deadline.days', count: 12 },
    ]);
  });

  it('reports value below the band', () => {
    const got = fitReasonFragments({
      sectorMatch: false,
      countryMatch: false,
      valueFit: 'below',
      deadlineDays: 0,
      hasDeadline: false,
      regionMatch: false,
      procedureMatch: false,
    });
    expect(got).toEqual([{ key: 'tenders.fit.reasonValueBelow' }]);
  });

  it('reports value above the band', () => {
    const got = fitReasonFragments({
      sectorMatch: false,
      countryMatch: false,
      valueFit: 'above',
      deadlineDays: 0,
      hasDeadline: false,
      regionMatch: false,
      procedureMatch: false,
    });
    expect(got).toEqual([{ key: 'tenders.fit.reasonValueAbove' }]);
  });

  it('never emits a fragment for valueFit "unknown"', () => {
    const got = fitReasonFragments({
      sectorMatch: false,
      countryMatch: false,
      valueFit: 'unknown',
      deadlineDays: 5,
      hasDeadline: true,
      regionMatch: false,
      procedureMatch: false,
    });
    expect(got).toEqual([{ key: 'tenders.deadline.days', count: 5 }]);
  });

  it('includes region and procedure match fragments, appended after sector/country/value/deadline', () => {
    const got = fitReasonFragments({
      sectorMatch: true,
      countryMatch: true,
      valueFit: 'in_band',
      deadlineDays: 12,
      hasDeadline: true,
      regionMatch: true,
      procedureMatch: true,
    });
    expect(got).toEqual([
      { key: 'tenders.fit.reasonSector' },
      { key: 'tenders.fit.reasonCountry' },
      { key: 'tenders.fit.reasonValueInBand' },
      { key: 'tenders.deadline.days', count: 12 },
      { key: 'tenders.fit.reasonRegion' },
      { key: 'tenders.fit.reasonProcedure' },
    ]);
  });

  it('emits only the region fragment when just regionMatch is true', () => {
    const got = fitReasonFragments({
      sectorMatch: false,
      countryMatch: false,
      valueFit: 'unknown',
      deadlineDays: 0,
      hasDeadline: false,
      regionMatch: true,
      procedureMatch: false,
    });
    expect(got).toEqual([{ key: 'tenders.fit.reasonRegion' }]);
  });

  it('emits only the procedure fragment when just procedureMatch is true', () => {
    const got = fitReasonFragments({
      sectorMatch: false,
      countryMatch: false,
      valueFit: 'unknown',
      deadlineDays: 0,
      hasDeadline: false,
      regionMatch: false,
      procedureMatch: true,
    });
    expect(got).toEqual([{ key: 'tenders.fit.reasonProcedure' }]);
  });
});
