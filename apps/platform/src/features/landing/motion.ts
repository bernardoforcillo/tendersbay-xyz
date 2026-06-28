import {
  type MotionStyle,
  type MotionValue,
  useMotionValue,
  useReducedMotion,
  useTransform,
} from 'motion/react';
import { type RefObject, useEffect } from 'react';

/**
 * Scroll-kinetic motion system for the landing.
 *
 * Progress is computed by hand from `getBoundingClientRect` in a passive,
 * rAF-throttled scroll listener into a plain `useMotionValue` — deliberately NOT
 * motion's `useScroll`. The native ScrollTimeline path threw "offsets must be
 * monotonically non-decreasing" for tall pinned targets, and its element-vs-window
 * fallback silently reported the wrong progress. A hand-rolled value is monotonic,
 * correct regardless of document height, and degrades to a no-op (static) in
 * layout-less environments (jsdom/SSR). Each helper returns `undefined` under
 * `prefers-reduced-motion` so callers render a static element. Transform/opacity
 * only (compositor-cheap).
 */

type Ref = RefObject<HTMLElement | null>;

/**
 * Progress 0→1 as the element's top edge travels from `startFrac` to `endFrac` of
 * the viewport height (1 = bottom edge, 0 = top edge, negative = above the top).
 */
function useViewProgress(ref: Ref, startFrac: number, endFrac: number): MotionValue<number> {
  const value = useMotionValue(0);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const denom = startFrac - endFrac || 1;
    let frame = 0;
    const compute = () => {
      frame = 0;
      const vh = window.innerHeight || 1;
      const topFrac = el.getBoundingClientRect().top / vh;
      value.set(Math.min(1, Math.max(0, (startFrac - topFrac) / denom)));
    };
    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(compute);
    };
    compute();
    window.addEventListener('scroll', onScroll, { passive: true });
    window.addEventListener('resize', onScroll);
    return () => {
      if (frame) cancelAnimationFrame(frame);
      window.removeEventListener('scroll', onScroll);
      window.removeEventListener('resize', onScroll);
    };
  }, [ref, startFrac, endFrac, value]);
  return value;
}

/**
 * Scroll-linked reveal: rises, fades, and settles as it scrolls into view —
 * fully revealed by the time its top reaches 60% of the viewport, so it is never
 * stuck dim at a resting position. `delay` staggers siblings (a grid cascades).
 */
export function useKineticReveal(ref: Ref, delay = 0): MotionStyle | undefined {
  const reduce = useReducedMotion();
  const p = useViewProgress(ref, 1, 0.6);
  const start = Math.min(delay, 0.3);
  const opacity = useTransform(p, [start, start + 0.4], [0, 1]);
  const y = useTransform(p, [start, 1], [56, 0]);
  const scale = useTransform(p, [start, 1], [0.97, 1]);
  return reduce ? undefined : { opacity, y, scale };
}

/** Parallax drift: content moves opposite the scroll across the section's pass. */
export function useParallax(ref: Ref, distance = 60): MotionValue<number> | undefined {
  const reduce = useReducedMotion();
  const p = useViewProgress(ref, 1, -0.2);
  const y = useTransform(p, [0, 1], [distance, -distance]);
  return reduce ? undefined : y;
}

/**
 * Kinetic block: a bold panel scales up and rises as it climbs toward centre,
 * then holds. For the standout colored blocks (hero panel, CTA card).
 */
export function useKineticBlock(ref: Ref): MotionStyle | undefined {
  const reduce = useReducedMotion();
  const p = useViewProgress(ref, 1, 0.4);
  const scale = useTransform(p, [0, 1], [0.94, 1]);
  const y = useTransform(p, [0, 1], [60, 0]);
  return reduce ? undefined : { scale, y };
}
