import { cn } from '../../../cn';

type Variant = 'text' | 'circle' | 'rect';

export type SkeletonProps = {
  className?: string;
  variant?: Variant;
};

const RADII: Record<Variant, string> = {
  text: 'rounded-md',
  circle: 'rounded-full',
  rect: 'rounded-xl',
};

export function Skeleton({ className, variant = 'text' }: SkeletonProps) {
  return (
    <div
      aria-hidden="true"
      className={cn('animate-pulse bg-cream-200', RADII[variant], className)}
    />
  );
}
