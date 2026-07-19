import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getCoverage = vi.fn();
vi.mock('~/lib/api/client', () => ({
  tenderClient: {
    getCoverage: (...args: unknown[]) => getCoverage(...args),
  },
}));

import { useCoverage } from './use-coverage';

describe('useCoverage', () => {
  beforeEach(() => {
    getCoverage.mockReset();
  });

  it('returns the fetched countries as a Set, filtered to EU', async () => {
    getCoverage.mockResolvedValue({ countries: ['IT', 'PL', 'XX'] });

    const { result } = renderHook(() => useCoverage());

    await waitFor(() => expect(result.current.has('IT')).toBe(true));
    expect(result.current.has('PL')).toBe(true);
    // Non-EU codes are filtered out so the marquee never lights a flag it can't render.
    expect(result.current.has('XX' as never)).toBe(false);
  });

  it('degrades to an empty Set on error', async () => {
    getCoverage.mockRejectedValue(new Error('down'));

    const { result } = renderHook(() => useCoverage());

    await waitFor(() => expect(getCoverage).toHaveBeenCalled());
    expect(result.current.size).toBe(0);
  });
});
