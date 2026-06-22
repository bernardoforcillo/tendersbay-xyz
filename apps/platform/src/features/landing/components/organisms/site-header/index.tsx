import { Link } from 'react-aria-components';
import { Logo } from '~/features/landing/components/atoms';
import { LanguageSwitcher, NavLinks } from '~/features/landing/components/molecules';

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-cream-300/70 bg-cream-100/80 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-6 py-4">
        <Link href="#top" aria-label="tendersbay" className="no-underline outline-none">
          <Logo />
        </Link>
        <NavLinks className="hidden md:flex" />
        <LanguageSwitcher />
      </div>
    </header>
  );
}
