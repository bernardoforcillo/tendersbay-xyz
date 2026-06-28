import { describe, expect, it } from 'vitest';
import enIe from '~/assets/locales/en-ie/common.json';
import itIt from '~/assets/locales/it-it/common.json';

const KEYS = ['title', 'body', 'statusAvailable', 'statusComingSoon', 'note'];

describe('landing.coverage copy', () => {
  it('has every coverage key in the source locale (en-ie)', () => {
    for (const key of KEYS) {
      expect((enIe.landing.coverage as Record<string, string>)[key]).toBeTruthy();
    }
  });

  it('is translated in it-it', () => {
    for (const key of KEYS) {
      expect((itIt.landing.coverage as Record<string, string>)[key]).toBeTruthy();
    }
    expect(itIt.landing.coverage.statusComingSoon).toBe('In arrivo');
  });
});
