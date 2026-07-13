import { cn } from '@tendersbay/components/core';
import { Link } from 'react-aria-components';
import { Icon, type IconName } from '~/features/landing/components/atoms';

type Social = { name: string; href: string; icon: IconName };

// Placeholder hrefs for the pre-launch teaser; swap to real URLs when available.
const SOCIALS: Social[] = [
  { name: 'GitHub', href: '#', icon: 'github' },
  { name: 'LinkedIn', href: '#', icon: 'linkedin' },
  { name: 'X', href: '#', icon: 'twitter' },
];

type SocialLinksProps = { label: string; className?: string };

export function SocialLinks({ label, className }: SocialLinksProps) {
  return (
    <nav aria-label={label} className={cn('flex items-center gap-2', className)}>
      {SOCIALS.map((social) => (
        <Link
          key={social.name}
          href={social.href}
          aria-label={social.name}
          className="inline-flex h-9 w-9 items-center justify-center rounded-full text-[18px] text-ink-300 outline-none ring-1 ring-white/10 transition-colors data-[hovered]:bg-white/5 data-[hovered]:text-brand-300 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-400"
        >
          <Icon name={social.icon} />
        </Link>
      ))}
    </nav>
  );
}
