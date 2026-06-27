import { motion } from 'motion/react';
import { type ReactNode, useRef } from 'react';
import { useKineticReveal } from '~/features/landing/motion';

type RevealProps = { children: ReactNode; delay?: number; className?: string };

/**
 * Scroll-linked reveal. The content rises, fades, and settles as it scrolls into
 * view — driven by scroll position, so it tracks the scrollbar. `delay` staggers
 * siblings (e.g. a card grid). Renders a plain `div` under reduced-motion.
 */
export function Reveal({ children, delay = 0, className }: RevealProps) {
  const ref = useRef<HTMLDivElement>(null);
  const style = useKineticReveal(ref, delay);
  if (!style) {
    return <div className={className}>{children}</div>;
  }
  return (
    <motion.div ref={ref} className={className} style={style}>
      {children}
    </motion.div>
  );
}
