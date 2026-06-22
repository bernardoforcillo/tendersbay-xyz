import type { ReactNode } from 'react';
import { Icon, type IconName } from '~/features/landing/components/atoms/icon';
import { cx } from '~/features/landing/cx';

type EyebrowProps = { icon?: IconName; children: ReactNode; className?: string };

export function Eyebrow({ icon, children, className }: EyebrowProps) {
  return (
    <span
      className={cx(
        'inline-flex items-center gap-2 rounded-full border border-brand-200 bg-brand-50 px-3 py-1.5',
        'text-xs font-bold uppercase tracking-wide text-brand-700',
        className,
      )}
    >
      {icon ? <Icon name={icon} className="text-[14px]" /> : null}
      {children}
    </span>
  );
}
