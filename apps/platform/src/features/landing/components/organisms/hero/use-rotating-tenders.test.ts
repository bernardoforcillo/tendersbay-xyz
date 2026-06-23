import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Tender } from '~/features/landing/components/atoms';
import { useRotatingTenders } from './use-rotating-tenders';

const tenders: Tender[] = [
  { id: 'a', entity: 'A', object: 'a', value: '€1', deadlineDays: 1, scoutCount: 1 },
  { id: 'b', entity: 'B', object: 'b', value: '€2', deadlineDays: 2, scoutCount: 2 },
];

function setReducedMotion(matches: boolean): void {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

afterEach(() => {
  vi.useRealTimers();
  setReducedMotion(false);
});

describe('useRotatingTenders', () => {
  it('advances to the next tender after the interval', () => {
    setReducedMotion(false);
    vi.useFakeTimers();
    const { result } = renderHook(() => useRotatingTenders(tenders, 1000));
    expect(result.current.tender.id).toBe('a');
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(result.current.tender.id).toBe('b');
  });

  it('does not advance when reduced motion is preferred', () => {
    setReducedMotion(true);
    vi.useFakeTimers();
    const { result } = renderHook(() => useRotatingTenders(tenders, 1000));
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(result.current.tender.id).toBe('a');
  });
});
