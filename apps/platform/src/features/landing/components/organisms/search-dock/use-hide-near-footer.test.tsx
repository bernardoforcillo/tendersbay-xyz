import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useHideNearFooter } from './use-hide-near-footer';

describe('useHideNearFooter', () => {
  it('defaults to visible (false) when the footer is absent', () => {
    const { result } = renderHook(() => useHideNearFooter('no-such-footer'));
    expect(result.current).toBe(false);
  });

  it('observes the footer element when present', () => {
    const footer = document.createElement('footer');
    footer.id = 'site-footer';
    document.body.appendChild(footer);
    const observe = vi.spyOn(IntersectionObserver.prototype, 'observe');

    renderHook(() => useHideNearFooter());

    expect(observe).toHaveBeenCalledTimes(1);
    observe.mockRestore();
    footer.remove();
  });
});
