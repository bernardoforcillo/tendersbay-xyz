import { describe, expect, it } from 'vitest';
import { fetchSampleTenders, initialSampleTenders, SAMPLE_TENDERS } from './sample-tenders';

const idsInPool = new Set(SAMPLE_TENDERS.map((t) => t.id));

describe('SAMPLE_TENDERS pool', () => {
  it('is a broad, well-shaped illustrative pool', () => {
    expect(SAMPLE_TENDERS.length).toBeGreaterThanOrEqual(12);
    // Unique ids, plausible single-to-low-double-digit deadlines, present fields.
    expect(new Set(idsInPool).size).toBe(SAMPLE_TENDERS.length);
    for (const t of SAMPLE_TENDERS) {
      expect(t.entity, `${t.id}.entity`).toBeTruthy();
      expect(t.object, `${t.id}.object`).toBeTruthy();
      expect(t.value, `${t.id}.value`).toBeTruthy();
      expect(t.deadlineDays, `${t.id}.deadlineDays`).toBeGreaterThan(0);
      expect(t.deadlineDays, `${t.id}.deadlineDays`).toBeLessThan(100);
    }
  });
});

describe('fetchSampleTenders', () => {
  it('resolves exactly `count` tenders, all drawn from the pool', async () => {
    const result = await fetchSampleTenders(3);
    expect(result).toHaveLength(3);
    for (const tender of result) {
      expect(idsInPool.has(tender.id)).toBe(true);
    }
  });

  it('returns unique tenders (no duplicates within a draw)', async () => {
    const result = await fetchSampleTenders(6);
    expect(new Set(result.map((t) => t.id)).size).toBe(result.length);
  });

  it('clamps a count larger than the pool without throwing', async () => {
    const result = await fetchSampleTenders(SAMPLE_TENDERS.length + 50);
    expect(result).toHaveLength(SAMPLE_TENDERS.length);
  });

  it('clamps a negative count to an empty list', async () => {
    await expect(fetchSampleTenders(-5)).resolves.toHaveLength(0);
  });
});

describe('initialSampleTenders', () => {
  it('returns a deterministic first-`count` slice for a stable first paint', () => {
    const first = initialSampleTenders(4);
    expect(first).toEqual(SAMPLE_TENDERS.slice(0, 4));
  });
});
