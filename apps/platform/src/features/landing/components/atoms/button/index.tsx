import { motion } from 'motion/react';
import type { ReactNode } from 'react';
import { Link } from 'react-aria-components';
import { cx } from '~/features/landing/cx';

type Variant = 'primary' | 'ghost' | 'invert' | 'text';
type ButtonProps = { href: string; variant?: Variant; children: ReactNode; className?: string };

const BASE =
  'inline-flex items-center gap-2 rounded-xl text-sm font-bold no-underline outline-none transition-colors ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2 ' +
  'data-[focus-visible]:ring-offset-cream-100';

const VARIANTS: Record<Variant, string> = {
  primary:
    'bg-brand-600 px-7 py-4 text-white shadow-lg shadow-brand-600/30 data-[hovered]:bg-brand-700',
  ghost: 'border border-cream-300 bg-white px-6 py-4 text-ink-800 data-[hovered]:border-cream-400',
  // White pill with deep-teal text — for use on bold teal/dark surfaces (e.g. the CTA band).
  invert:
    'bg-white px-7 py-4 text-brand-700 shadow-lg shadow-brand-950/20 data-[hovered]:bg-cream-100 data-[focus-visible]:ring-offset-brand-700',
  text: 'px-2 py-2 font-semibold text-brand-700 data-[hovered]:text-brand-800',
};

const MotionLink = motion.create(Link);

export function Button({ href, variant = 'primary', children, className }: ButtonProps) {
  const classes = cx(BASE, VARIANTS[variant], className);
  if (variant === 'primary' || variant === 'invert') {
    return (
      <MotionLink href={href} className={classes} whileHover={{ y: -2 }} whileTap={{ scale: 0.98 }}>
        {children}
      </MotionLink>
    );
  }
  return (
    <Link href={href} className={classes}>
      {children}
    </Link>
  );
}
