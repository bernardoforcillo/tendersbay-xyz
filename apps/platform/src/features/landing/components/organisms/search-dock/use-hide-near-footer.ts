import { useEffect, useState } from 'react';

/**
 * Returns `true` while the element with id `footerId` is intersecting the
 * viewport, so a fixed overlay can fade out and stop covering the footer.
 * Defaults to `false` (visible) when the element or IntersectionObserver
 * is unavailable.
 */
export function useHideNearFooter(footerId = 'site-footer'): boolean {
  const [hidden, setHidden] = useState(false);

  useEffect(() => {
    if (typeof IntersectionObserver === 'undefined') return;
    const footer = document.getElementById(footerId);
    if (!footer) return;

    const observer = new IntersectionObserver(
      ([entry]) => setHidden(entry?.isIntersecting ?? false),
      { rootMargin: '0px 0px -8% 0px' },
    );
    observer.observe(footer);
    return () => observer.disconnect();
  }, [footerId]);

  return hidden;
}
