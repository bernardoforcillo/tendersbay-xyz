import { Icon, type IconName } from '~/features/landing/components/atoms';
import { cx } from '~/features/landing/cx';

type ValueCardProps = { icon: IconName; title: string; body: string; className?: string };

export function ValueCard({ icon, title, body, className }: ValueCardProps) {
  return (
    <div
      className={cx(
        'rounded-2xl border border-cream-300 bg-white p-6 transition-shadow hover:shadow-lg hover:shadow-ink-900/5',
        className,
      )}
    >
      <span className="inline-flex h-11 w-11 items-center justify-center rounded-xl bg-brand-50 text-[20px] text-brand-600">
        <Icon name={icon} />
      </span>
      <h3 className="mt-4 text-lg font-bold text-ink-900">{title}</h3>
      <p className="mt-2 text-sm leading-relaxed text-ink-600">{body}</p>
    </div>
  );
}
