import type { ReactNode } from 'react';
import { Switch as RACSwitch, type SwitchProps as RACSwitchProps } from 'react-aria-components';
import { cn } from '../../../cn';

export type SwitchProps = Omit<RACSwitchProps, 'className' | 'children'> & {
  children?: ReactNode;
  className?: string;
};

export function Switch({ children, className, ...props }: SwitchProps) {
  return (
    <RACSwitch
      {...props}
      className={cn(
        'group inline-flex items-center gap-2 text-sm font-medium text-ink-900 outline-none',
        'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2',
        'data-[focus-visible]:ring-offset-cream-100',
        className,
      )}
    >
      <div className="flex h-6 w-10 items-center rounded-full bg-cream-300 transition-colors duration-150 group-data-[selected]:bg-brand-600">
        <span className="h-5 w-5 translate-x-0.5 rounded-full bg-white shadow-soft transition-transform duration-150 group-data-[selected]:translate-x-[18px]" />
      </div>
      {children}
    </RACSwitch>
  );
}
