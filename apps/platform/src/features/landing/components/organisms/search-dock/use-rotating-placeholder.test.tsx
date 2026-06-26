import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useRotatingPlaceholder } from './use-rotating-placeholder';

describe('useRotatingPlaceholder', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns the first example initially', () => {
    const { result } = renderHook(() => useRotatingPlaceholder(['a', 'b', 'c'], true));
    expect(result.current.example).toBe('a');
  });

  it('advances to the next example after the interval', () => {
    const { result } = renderHook(() => useRotatingPlaceholder(['a', 'b', 'c'], true));
    act(() => {
      vi.advanceTimersByTime(2800);
    });
    expect(result.current.example).toBe('b');
  });

  it('does not rotate when disabled', () => {
    const { result } = renderHook(() => useRotatingPlaceholder(['a', 'b', 'c'], false));
    act(() => {
      vi.advanceTimersByTime(10000);
    });
    expect(result.current.example).toBe('a');
  });

  it('does not rotate when there is a single example', () => {
    const { result } = renderHook(() => useRotatingPlaceholder(['only'], true));
    act(() => {
      vi.advanceTimersByTime(10000);
    });
    expect(result.current.example).toBe('only');
  });
});
