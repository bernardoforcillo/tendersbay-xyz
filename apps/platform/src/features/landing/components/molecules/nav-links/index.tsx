import { cn } from '@tendersbay/components/core';
import { Link } from 'react-aria-components';
import { useTranslation } from 'react-i18next';

const LINKS = [
  { href: '#problem', key: 'landing.nav.problem' },
  { href: '#agents', key: 'landing.nav.agents' },
  { href: '#vision', key: 'landing.nav.vision' },
] as const;

export function NavLinks({ className }: { className?: string }) {
  const { t } = useTranslation();
  return (
    <nav className={cn('flex items-center gap-6', className)}>
      {LINKS.map((link) => (
        <Link
          key={link.href}
          href={link.href}
          className="group relative text-sm font-medium text-ink-700 no-underline outline-none transition-colors data-[hovered]:text-ink-900 data-[focus-visible]:text-ink-900"
        >
          {t(link.key)}
          <span
            aria-hidden="true"
            className="absolute -bottom-1.5 left-0 h-[2px] w-full origin-left scale-x-0 rounded-full bg-brand-600 transition-transform duration-300 ease-out group-data-[hovered]:scale-x-100 group-data-[focus-visible]:scale-x-100"
          />
        </Link>
      ))}
    </nav>
  );
}
