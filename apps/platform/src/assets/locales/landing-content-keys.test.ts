import { describe, expect, it } from 'vitest';

type Card = { title?: string; body?: string };
type Landing = {
  audience?: { title?: string; items?: Card[] };
  assurance?: { eyebrow?: string; title?: string; items?: Card[] };
};

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { landing: Landing } }
>;

const entries = Object.entries(modules);

const cardsAreFilled = (cards: Card[] | undefined, expected: number) =>
  Array.isArray(cards) &&
  cards.length === expected &&
  cards.every((c) => Boolean(c.title) && Boolean(c.body));

describe('landing persona + assurance locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s has a 3-card persona audience block', (_path, mod) => {
    const audience = mod.default.landing.audience;
    expect(audience?.title, 'audience.title').toBeTruthy();
    expect(cardsAreFilled(audience?.items, 3), 'audience.items (3 filled cards)').toBe(true);
  });

  it.each(entries)('%s has a 4-card assurance block', (_path, mod) => {
    const assurance = mod.default.landing.assurance;
    expect(assurance?.eyebrow, 'assurance.eyebrow').toBeTruthy();
    expect(assurance?.title, 'assurance.title').toBeTruthy();
    expect(cardsAreFilled(assurance?.items, 4), 'assurance.items (4 filled cards)').toBe(true);
  });
});
