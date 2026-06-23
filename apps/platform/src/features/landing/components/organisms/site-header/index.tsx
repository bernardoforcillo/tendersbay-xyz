import { Link } from 'react-aria-components';
import { Logo } from '~/features/landing/components/atoms';
import { LanguageSwitcher, NavLinks } from '~/features/landing/components/molecules';

export function SiteHeader() {
  return (
    <header className="absolute inset-x-0 top-0 z-50">
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
