import type { ReactNode } from 'react';
import { Button as RACButton, type ButtonProps as RACButtonProps } from 'react-aria-components';
import { cn } from '../../../cn';

type Variant = 'primary' | 'ghost' | 'quiet' | 'danger';
type Size = 'md' | 'lg';

export type ButtonProps = Omit<RACButtonProps, 'className' | 'children'> & {
  variant?: Variant;
  size?: Size;
  className?: string;
  children?: ReactNode;
  isLoading?: boolean;
};

const BASE =
  'inline-flex items-center justify-center gap-2 rounded-xl font-semibold outline-none ' +
  'transition-colors duration-150 ' +
  'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2 ' +
  'data-[focus-visible]:ring-offset-cream-100 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50';

const VARIANTS: Record<Variant, string> = {
  primary: 'bg-brand-600 text-white data-[hovered]:bg-brand-700 data-[pressed]:bg-brand-800',
  ghost:
    'border border-cream-300 bg-white text-ink-800 data-[hovered]:border-cream-400 data-[pressed]:bg-cream-100',
  quiet: 'text-ink-700 data-[hovered]:bg-cream-200 data-[pressed]:bg-cream-300',
  danger: 'bg-red-600 text-white data-[hovered]:bg-red-700 data-[pressed]:bg-red-800',
};

// Click targets stay ≥40px — a fixed cognitive rule of the design system.
const SIZES: Record<Size, string> = {
  md: 'h-10 px-4 text-sm',
  lg: 'h-12 px-6 text-base',
};

function Spinner() {
  return (
    <svg className="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
      />
    </svg>
  );
}

export function Button({
  variant = 'primary',
  size = 'md',
  isLoading = false,
  className,
  children,
  ...props
}: ButtonProps) {
  return (
    <RACButton
      {...props}
      isDisabled={props.isDisabled || isLoading}
      className={cn(BASE, VARIANTS[variant], SIZES[size], className)}
    >
      {isLoading && <Spinner />}
      <span aria-hidden={isLoading}>{children}</span>
    </RACButton>
  );
}
