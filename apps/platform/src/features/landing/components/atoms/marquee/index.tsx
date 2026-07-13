import { cn } from '@tendersbay/components/core';
import {
  motion,
  useAnimationFrame,
  useInView,
  useMotionValue,
  useReducedMotion,
} from 'motion/react';
import {
  Children,
  cloneElement,
  isValidElement,
  type ReactElement,
  type ReactNode,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';

/** Inter-item / inter-track gap in px — must match the `gap-3` utility (0.75rem). */
const GAP_PX = 12;

type MarqueeProps = {
  children: ReactNode;
  /** Scroll right-to-left by default; set to reverse the direction. */
  reverse?: boolean;
  /** Seconds for one full loop. Higher = slower. */
  durationSec?: number;
  className?: string;
};

/**
 * Seamless infinite horizontal marquee driven by `motion`'s `useAnimationFrame`.
 * Renders the children twice; both tracks share one motion value translated in
 * px and wrapped by (track width + gap), so the loop is gapless. Pauses on
 * hover/focus, idles when off-screen, and stays still under reduced-motion. The
 * duplicate track is `inert` + `aria-hidden` — visual filler only.
 */
export function Marquee({ children, reverse, durationSec = 40, className }: MarqueeProps) {
  const reduce = useReducedMotion();
  const containerRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const inView = useInView(containerRef, { margin: '200px' });
  const x = useMotionValue(0);
  const [paused, setPaused] = useState(false);
  const [loopWidth, setLoopWidth] = useState(0);

  useLayoutEffect(() => {
    const el = trackRef.current;
    if (!el) return;
    const measure = () => setLoopWidth(el.offsetWidth + GAP_PX);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  useAnimationFrame((_, delta) => {
    if (reduce || paused || !inView || loopWidth === 0) return;
    const pxPerSecond = loopWidth / durationSec;
    let next = x.get() + (reverse ? 1 : -1) * pxPerSecond * (delta / 1000);
    if (next <= -loopWidth) next += loopWidth;
    else if (next >= 0) next -= loopWidth;
    x.set(next);
  });

  const track = 'flex w-max shrink-0 items-stretch gap-3';
  // The duplicate copy stays mouse-hoverable (so pause + card work on every
  // visible tile) but is hidden from screen readers and removed from the tab
  // order — hence `aria-hidden` on the track and `decorative` on each child.
  const duplicate = Children.map(children, (child) =>
    isValidElement(child)
      ? cloneElement(child as ReactElement<{ decorative?: boolean }>, { decorative: true })
      : child,
  );

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: pause-on-hover/focus is a non-essential enhancement; the marquee items are individually focusable buttons.
    <div
      ref={containerRef}
      className={cn('flex gap-3 overflow-hidden', className)}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocusCapture={() => setPaused(true)}
      onBlurCapture={() => setPaused(false)}
    >
      <motion.div ref={trackRef} className={track} style={{ x }}>
        {children}
      </motion.div>
      <motion.div className={track} style={{ x }} aria-hidden="true">
        {duplicate}
      </motion.div>
    </div>
  );
}
