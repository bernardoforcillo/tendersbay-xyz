import { describe, expect, it } from 'vitest';
import { countryFlag, countryName } from './country';

describe('countryFlag', () => {
  it('resolves the alpha-3 code the backend stores', () => {
    expect(countryFlag('ITA')).not.toBeNull();
    expect(countryFlag('PRT')).not.toBeNull();
  });

  it('resolves a bare alpha-2 code too', () => {
    expect(countryFlag('IT')).not.toBeNull();
  });

  it('resolves the EU quirk codes (EL for Greece, UK for Britain)', () => {
    expect(countryFlag('EL')).toBe(countryFlag('GR'));
    expect(countryFlag('UK')).toBe(countryFlag('GB'));
  });

  it('is case- and whitespace-insensitive', () => {
    expect(countryFlag('  ita ')).toBe(countryFlag('ITA'));
  });

  it('returns null for a code we do not carry a flag for', () => {
    expect(countryFlag('ZZZ')).toBeNull();
    expect(countryFlag('')).toBeNull();
  });
});

describe('countryName', () => {
  it('localises the country name from an alpha-3 code', () => {
    expect(countryName('ITA', 'en')).toBe('Italy');
    expect(countryName('ITA', 'it')).toBe('Italia');
  });

  it('falls back to the raw code when unknown', () => {
    expect(countryName('ZZZ', 'en')).toBe('ZZZ');
  });
});
