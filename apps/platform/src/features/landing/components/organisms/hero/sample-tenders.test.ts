import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchSampleTenders, initialSampleTenders, SAMPLE_TENDERS } from './sample-tenders';

const searchTenders = vi.fn();
vi.mock('~/lib/api/client', () => ({
  tenderClient: { searchTenders: (...args: unknown[]) => searchTenders(...args) },
}));

const idsInPool = new Set(SAMPLE_TENDERS.map((t) => t.id));

/** A minimal backend row carrying just the fields `toTender` reads. */
function backendTender(id: string) {
  return {
    id,
    title: `Real tender ${id}`,
    buyerName: `Buyer ${id}`,
    country: 'ITA',
    value: 1_000_000n,
    currency: 'EUR',
    deadline: '2099-01-01T00:00:00Z',
  };
}

beforeEach(() => {
  searchTenders.mockReset();
  // Default: the backend has nothing yet → the deck falls back to the curated pool.
  searchTenders.mockResolvedValue({ results: [], hasMore: false });
});

describe('SAMPLE_TENDERS pool', () => {
  it('is a broad, well-shaped illustrative pool', () => {
    expect(SAMPLE_TENDERS.length).toBeGreaterThanOrEqual(12);
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

describe('fetchSampleTenders — live results', () => {
  it('maps backend tenders to the card shape when the search returns rows', async () => {
    searchTenders.mockResolvedValueOnce({
      results: [backendTender('a'), backendTender('b'), backendTender('c')],
      hasMore: false,
    });
    const result = await fetchSampleTenders(3);

    expect(searchTenders).toHaveBeenCalledWith({ query: '', limit: 3, offset: 0 });
    expect(result).toHaveLength(3);
    expect(result[0]).toMatchObject({ id: 'a', entity: 'Buyer a', object: 'Real tender a' });
    // Value formatted, deadline mapped to a positive day count, and not from the pool.
    expect(result[0]?.value).toContain('1');
    expect(result[0]?.deadlineDays).toBeGreaterThan(0);
    expect(idsInPool.has(result[0]?.id ?? '')).toBe(false);
  });

  it('caps live results at `count` rows', async () => {
    searchTenders.mockResolvedValueOnce({
      results: Array.from({ length: 10 }, (_, i) => backendTender(String(i))),
      hasMore: true,
    });
    expect(await fetchSampleTenders(3)).toHaveLength(3);
  });
});

describe('fetchSampleTenders — curated fallback', () => {
  it('falls back to the pool when the backend returns nothing', async () => {
    const result = await fetchSampleTenders(3); // beforeEach → empty results
    expect(result).toHaveLength(3);
    for (const tender of result) expect(idsInPool.has(tender.id)).toBe(true);
  });

  it('falls back to the pool when the search rejects', async () => {
    searchTenders.mockRejectedValueOnce(new Error('network down'));
    const result = await fetchSampleTenders(3);
    expect(result).toHaveLength(3);
    for (const tender of result) expect(idsInPool.has(tender.id)).toBe(true);
  });

  it('returns unique tenders (no duplicates within a draw)', async () => {
    const result = await fetchSampleTenders(6);
    expect(new Set(result.map((t) => t.id)).size).toBe(result.length);
  });

  it('clamps a count larger than the pool without throwing', async () => {
    const result = await fetchSampleTenders(SAMPLE_TENDERS.length + 50);
    expect(result).toHaveLength(SAMPLE_TENDERS.length);
  });

  it('clamps a negative count to an empty list without hitting the backend', async () => {
    await expect(fetchSampleTenders(-5)).resolves.toHaveLength(0);
    expect(searchTenders).not.toHaveBeenCalled();
  });
});

describe('initialSampleTenders', () => {
  it('returns a deterministic first-`count` slice for a stable first paint', () => {
    const first = initialSampleTenders(4);
    expect(first).toEqual(SAMPLE_TENDERS.slice(0, 4));
  });
});
