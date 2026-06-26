import { motion, useReducedMotion } from 'motion/react';
import { Link } from 'react-aria-components';
import { Logo } from '~/features/landing/components/atoms';
import { LanguageSwitcher, NavLinks } from '~/features/landing/components/molecules';
import { useScrolled } from './use-scrolled';

// Full shadow string (matches --shadow-soft-lg) so motion can animate it.
// NO_SHADOW keeps the same two-layer structure (zero alpha) so motion
// interpolates layer-by-layer and the shadow fades in instead of snapping.
const PILL_SHADOW = '0 4px 8px rgba(19, 50, 44, 0.06), 0 22px 48px rgba(19, 50, 44, 0.12)';
const NO_SHADOW = '0 4px 8px rgba(19, 50, 44, 0), 0 22px 48px rgba(19, 50, 44, 0)';

export function SiteHeader() {
  const scrolled = useScrolled();
  const reduce = useReducedMotion();

  return (
    <header className="fixed inset-x-0 top-0 z-50 flex justify-center px-4 pt-3">
      <motion.div
        className="flex w-full items-center justify-between gap-4 border px-6 backdrop-blur-md"
        initial={false}
        animate={{
          maxWidth: scrolled ? 768 : 1152,
          paddingTop: scrolled ? 10 : 16,
          paddingBottom: scrolled ? 10 : 16,
          borderRadius: scrolled ? 9999 : 16,
          // cream-50 at 0.8 / 0 alpha — same rgb so the alpha animates smoothly.
          backgroundColor: scrolled ? 'rgba(253, 251, 247, 0.8)' : 'rgba(253, 251, 247, 0)',
          // cream-300 at 0.7 / 0 alpha.
          borderColor: scrolled ? 'rgba(236, 226, 210, 0.7)' : 'rgba(236, 226, 210, 0)',
          boxShadow: scrolled ? PILL_SHADOW : NO_SHADOW,
        }}
        transition={reduce ? { duration: 0 } : { duration: 0.35, ease: [0.22, 1, 0.36, 1] }}
      >
        <Link href="#top" aria-label="tendersbay" className="no-underline outline-none">
          <Logo />
        </Link>
        <NavLinks className="hidden md:flex" />
        <LanguageSwitcher />
      </motion.div>
    </header>
  );
}
