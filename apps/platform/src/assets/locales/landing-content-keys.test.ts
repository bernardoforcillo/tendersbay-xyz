import { describe, expect, it } from 'vitest';

type Card = { title?: string; body?: string };
type AgentItem = { time?: string; title?: string; body?: string };
type Stat = { value?: string; label?: string };
type Landing = {
  proof?: { lead?: string; items?: Stat[]; source?: string };
  agents?: { lead?: string; title?: string; items?: AgentItem[] };
  audience?: { title?: string; items?: Card[] };
  assurance?: { eyebrow?: string; title?: string; items?: Card[] };
  cta?: { title?: string; body?: string; button?: string };
};

// The overnight-hook timeline is universal (24h clock), like the proof stats:
// the timestamps stay identical across every locale — only prose is translated.
const AGENT_TIMES = ['02:14', '05:30', '07:00'];

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { landing: Landing } }
>;

const entries = Object.entries(modules);

const cardsAreFilled = (cards: Card[] | undefined, expected: number) =>
  Array.isArray(cards) &&
  cards.length === expected &&
  cards.every((c) => Boolean(c.title) && Boolean(c.body));

const statsAreFilled = (stats: Stat[] | undefined, expected: number) =>
  Array.isArray(stats) &&
  stats.length === expected &&
  stats.every((s) => Boolean(s.value) && Boolean(s.label));

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
    expect(assurance?.title, 'assurance.title').toBeTruthy();
    expect(cardsAreFilled(assurance?.items, 4), 'assurance.items (4 filled cards)').toBe(true);
  });
});

describe('landing proof-strip + agents-lead locale keys', () => {
  it.each(entries)('%s has a sourced 3-stat proof block', (_path, mod) => {
    const proof = mod.default.landing.proof;
    expect(typeof proof?.lead, 'proof.lead is a string').toBe('string');
    expect(proof?.lead, 'proof.lead non-empty').toBeTruthy();
    expect(statsAreFilled(proof?.items, 3), 'proof.items (3 stats with value + label)').toBe(true);
    expect(typeof proof?.source, 'proof.source is a string').toBe('string');
    expect(proof?.source, 'proof.source non-empty').toBeTruthy();
  });

  it.each(entries)('%s has an agents hook block (lead + timestamped items)', (_path, mod) => {
    const agents = mod.default.landing.agents;
    expect(typeof agents?.lead, 'agents.lead is a string').toBe('string');
    expect(agents?.lead, 'agents.lead non-empty').toBeTruthy();
    expect(agents?.title, 'agents.title non-empty').toBeTruthy();
    const items = agents?.items;
    expect(Array.isArray(items) && items.length === 3, 'agents.items has 3 cards').toBe(true);
    expect(
      items?.every((it) => Boolean(it.time) && Boolean(it.title) && Boolean(it.body)),
      'agents.items each have time + title + body',
    ).toBe(true);
    // Timestamps form the same 02:14 → 05:30 → 07:00 overnight timeline in every locale.
    expect(
      items?.map((it) => it.time),
      'agents timeline is universal',
    ).toEqual(AGENT_TIMES);
  });

  it.each(entries)('%s has a signup-oriented cta button', (_path, mod) => {
    const cta = mod.default.landing.cta;
    expect(cta?.button, 'cta.button non-empty').toBeTruthy();
  });
});
