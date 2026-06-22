import { Link } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { cx } from '~/features/landing/cx';

const LINKS = [
  { href: '#problem', key: 'landing.nav.problem' },
  { href: '#agents', key: 'landing.nav.agents' },
  { href: '#vision', key: 'landing.nav.vision' },
] as const;

export function NavLinks({ className }: { className?: string }) {
  const { t } = useTranslation();
  return (
    <nav className={cx('flex items-center gap-6', className)}>
      {LINKS.map((link) => (
        <Link
          key={link.href}
          href={link.href}
          className="text-sm font-medium text-ink-600 no-underline outline-none data-[hovered]:text-ink-900 data-[focus-visible]:text-ink-900"
        >
          {t(link.key)}
        </Link>
      ))}
    </nav>
  );
}
