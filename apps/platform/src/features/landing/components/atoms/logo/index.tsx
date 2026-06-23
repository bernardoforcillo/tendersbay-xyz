import { cx } from '~/features/landing/cx';

export function Logo({ className }: { className?: string }) {
  return (
    <span className={cx('text-lg font-extrabold tracking-tight text-ink-900', className)}>
      tenders<span className="text-brand-600">bay</span>
    </span>
  );
}
