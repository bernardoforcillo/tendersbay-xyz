import {
  type MotionStyle,
  type MotionValue,
  useReducedMotion,
  useScroll,
  useTransform,
} from 'motion/react';
import { type RefObject, useEffect, useState } from 'react';

/**
 * Scroll-kinetic motion system for the landing.
 *
 * Every helper is driven by scroll *position* (not a one-shot timer), so motion
 * tracks the scrollbar — content reacts as you scroll. Each helper returns
 * `undefined` under `prefers-reduced-motion`, so callers render a static element
 * with no transforms. Only `transform`/`opacity` are animated (compositor-cheap).
 */

type Ref = RefObject<HTMLElement | null>;

/**
 * Enable element-target scroll tracking only once the element has a real layout
 * box. Without this, `useScroll({ target })` throws "Target ref is defined but
 * not hydrated" in layout-less environments (jsdom/SSR); falling back to the
 * window scroll there keeps tests and server render clean.
 */
function useScrollReady(ref: Ref): boolean {
  const [ready, setReady] = useState(false);
  useEffect(() => {
    if ((ref.current?.getBoundingClientRect().height ?? 0) > 0) setReady(true);
  }, [ref]);
  return ready;
}

/**
 * Scroll-linked reveal: as the element scrolls up into view it rises, fades in,
 * and settles. `delay` staggers siblings by shifting the start of the reveal band
 * (so a grid cascades). Drop into any element via a `motion` component's `style`.
 */
export function useKineticReveal(ref: Ref, delay = 0): MotionStyle | undefined {
  const reduce = useReducedMotion();
  const ready = useScrollReady(ref);
  const { scrollYProgress } = useScroll({
    target: ready ? ref : undefined,
    offset: ['start end', 'start 0.55'],
  });
  const start = Math.min(delay, 0.3);
  const opacity = useTransform(scrollYProgress, [start, start + 0.5], [0, 1]);
  const y = useTransform(scrollYProgress, [start, 1], [64, 0]);
  const scale = useTransform(scrollYProgress, [start, 1], [0.96, 1]);
  return reduce ? undefined : { opacity, y, scale };
}

/**
 * Parallax drift: content moves opposite the scroll across the section's pass,
 * for depth. `distance` is half the total px travel (enters +distance, exits
 * -distance). Apply to a `motion` element's `style.y`.
 */
export function useParallax(ref: Ref, distance = 60): MotionValue<number> | undefined {
  const reduce = useReducedMotion();
  const ready = useScrollReady(ref);
  const { scrollYProgress } = useScroll({
    target: ready ? ref : undefined,
    offset: ['start end', 'end start'],
  });
  const y = useTransform(scrollYProgress, [0, 1], [distance, -distance]);
  return reduce ? undefined : y;
}

/**
 * Kinetic block: a bold panel scales up and rises as it climbs to centre, then
 * holds. For the standout colored blocks (hero panel, CTA card).
 */
export function useKineticBlock(ref: Ref): MotionStyle | undefined {
  const reduce = useReducedMotion();
  const ready = useScrollReady(ref);
  const { scrollYProgress } = useScroll({
    target: ready ? ref : undefined,
    offset: ['start end', 'center center'],
  });
  const scale = useTransform(scrollYProgress, [0, 1], [0.94, 1]);
  const y = useTransform(scrollYProgress, [0, 1], [60, 0]);
  return reduce ? undefined : { scale, y };
}
