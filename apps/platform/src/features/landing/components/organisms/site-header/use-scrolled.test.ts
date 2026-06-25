import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useScrolled } from './use-scrolled';

function setScrollY(value: number) {
  Object.defineProperty(window, 'scrollY', { value, writable: true, configurable: true });
}

describe('useScrolled', () => {
  beforeEach(() => {
    // Run rAF callbacks synchronously so scroll updates are deterministic.
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      cb(0);
      return 0;
    });
    setScrollY(0);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    setScrollY(0);
  });

  it('is false at the top of the page', () => {
    const { result } = renderHook(() => useScrolled(32));
    expect(result.current).toBe(false);
  });

  it('becomes true after scrolling past the threshold', () => {
    const { result } = renderHook(() => useScrolled(32));
    act(() => {
      setScrollY(100);
      window.dispatchEvent(new Event('scroll'));
    });
    expect(result.current).toBe(true);
  });

  it('returns to false when scrolled back above the threshold', () => {
    const { result } = renderHook(() => useScrolled(32));
    act(() => {
      setScrollY(100);
      window.dispatchEvent(new Event('scroll'));
    });
    act(() => {
      setScrollY(0);
      window.dispatchEvent(new Event('scroll'));
    });
    expect(result.current).toBe(false);
  });
});
