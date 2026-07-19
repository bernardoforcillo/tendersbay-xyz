import { Link as RouterLink } from '@tanstack/react-router';
import { cn } from '@tendersbay/components/core';
import { motion } from 'motion/react';
import type { ComponentType, ReactNode } from 'react';
import { Link } from 'react-aria-components';

type Variant = 'primary' | 'ghost' | 'invert' | 'text';

type CommonProps = { variant?: Variant; children: ReactNode; className?: string };
// External / in-page anchor (react-aria Link) — hash links, mailto, etc.
type AnchorProps = CommonProps & { href: string; to?: undefined; params?: undefined };
// Internal route (TanStack Router Link) — client-side navigation to an app route.
type RouteProps = CommonProps & {
  to: string;
  params?: Record<string, string>;
  href?: undefined;
};
type ButtonProps = AnchorProps | RouteProps;

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

// A TanStack Router Link renders a plain <a> and does not emit react-aria's
// data-[hovered] / data-[focus-visible] state attributes, so mirror the hover +
// focus styling with native pseudo-classes for the router-link path only.
const ROUTER_STATE: Record<Variant, string> = {
  primary:
    'hover:bg-brand-700 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 focus-visible:ring-offset-cream-100',
  ghost:
    'hover:border-cream-400 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 focus-visible:ring-offset-cream-100',
  invert:
    'hover:bg-cream-100 focus-visible:ring-2 focus-visible:ring-white focus-visible:ring-offset-2 focus-visible:ring-offset-brand-700',
  text: 'hover:text-brand-800 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 focus-visible:ring-offset-cream-100',
};

const MotionLink = motion.create(Link);

// Treat the router Link as a simple anchor-like component so it composes with
// motion and this atom's props without threading TanStack's route generics.
type RouterLinkLikeProps = {
  to: string;
  params?: Record<string, string>;
  className?: string;
  children?: ReactNode;
};
const RouterLinkLike = RouterLink as unknown as ComponentType<RouterLinkLikeProps>;
const MotionRouterLink = motion.create(RouterLinkLike);

export function Button(props: ButtonProps) {
  const { variant = 'primary', children, className } = props;
  const lifts = variant === 'primary' || variant === 'invert';

  // Internal app route → TanStack Router Link (client-side navigation).
  if (props.to !== undefined) {
    const classes = cn(BASE, VARIANTS[variant], ROUTER_STATE[variant], className);
    if (lifts) {
      return (
        <MotionRouterLink
          to={props.to}
          params={props.params}
          className={classes}
          whileHover={{ y: -2 }}
          whileTap={{ scale: 0.98 }}
        >
          {children}
        </MotionRouterLink>
      );
    }
    return (
      <RouterLinkLike to={props.to} params={props.params} className={classes}>
        {children}
      </RouterLinkLike>
    );
  }

  // External / in-page anchor → react-aria Link.
  const classes = cn(BASE, VARIANTS[variant], className);
  if (lifts) {
    return (
      <MotionLink
        href={props.href}
        className={classes}
        whileHover={{ y: -2 }}
        whileTap={{ scale: 0.98 }}
      >
        {children}
      </MotionLink>
    );
  }
  return (
    <Link href={props.href} className={classes}>
      {children}
    </Link>
  );
}
