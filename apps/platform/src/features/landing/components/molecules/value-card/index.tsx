import { Icon, type IconName } from '~/features/landing/components/atoms';
import { cx } from '~/features/landing/cx';

type Tone = 'solution' | 'muted';
type ValueCardProps = {
  icon: IconName;
  title: string;
  body: string;
  tone?: Tone;
  className?: string;
};

const SURFACE: Record<Tone, string> = {
  solution: 'border-cream-200 bg-white shadow-soft hover:shadow-soft-lg',
  muted: 'border-cream-300 bg-cream-50 shadow-soft hover:shadow-soft-md',
};

const ICON_TILE: Record<Tone, string> = {
  solution: 'bg-brand-50 text-brand-600 ring-brand-100 group-hover:bg-brand-100',
  muted: 'bg-cream-200/70 text-ink-500 ring-cream-300 group-hover:bg-cream-200',
};

export function ValueCard({ icon, title, body, tone = 'solution', className }: ValueCardProps) {
  return (
    <div
      className={cx(
        'group h-full rounded-3xl border p-7 transition-all duration-300 hover:-translate-y-1',
        SURFACE[tone],
        className,
      )}
    >
      <span
        className={cx(
          'inline-flex h-12 w-12 items-center justify-center rounded-2xl text-[21px] ring-1 transition-colors duration-300',
          ICON_TILE[tone],
        )}
      >
        <Icon name={icon} />
      </span>
      <h3 className="mt-5 font-display text-xl text-ink-900">{title}</h3>
      <p className="mt-2.5 text-[15px] leading-relaxed text-ink-600">{body}</p>
    </div>
  );
}
