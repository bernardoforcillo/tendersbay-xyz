import { useEffect, useState } from 'react';

/**
 * Returns `true` once the window has scrolled past `threshold` pixels.
 * Passive scroll listener, throttled with requestAnimationFrame; SSR-safe
 * (defaults `false` until mounted).
 */
export function useScrolled(threshold = 32): boolean {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    let ticking = false;
    const update = () => {
      ticking = false;
      setScrolled(window.scrollY > threshold);
    };
    const onScroll = () => {
      if (ticking) return;
      ticking = true;
      window.requestAnimationFrame(update);
    };

    update(); // sync with the current scroll position on mount
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, [threshold]);

  return scrolled;
}
