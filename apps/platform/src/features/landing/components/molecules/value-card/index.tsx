import { Icon, type IconName } from '~/features/landing/components/atoms';
import { cx } from '~/features/landing/cx';

type ValueCardProps = { icon: IconName; title: string; body: string; className?: string };

export function ValueCard({ icon, title, body, className }: ValueCardProps) {
  return (
    <div
      className={cx(
        'group rounded-3xl border border-cream-200 bg-white p-7 shadow-soft transition-all duration-300 hover:-translate-y-1 hover:shadow-soft-lg',
        className,
      )}
    >
      <span className="inline-flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-50 text-[21px] text-brand-600 ring-1 ring-brand-100 transition-colors duration-300 group-hover:bg-brand-100">
        <Icon name={icon} />
      </span>
      <h3 className="mt-5 font-display text-xl text-ink-900">{title}</h3>
      <p className="mt-2.5 text-[15px] leading-relaxed text-ink-600">{body}</p>
    </div>
  );
}
