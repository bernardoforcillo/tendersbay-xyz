import type { ReactNode } from 'react';
import { cn } from '../../../cn';

export type EmptyStateProps = {
  icon?: ReactNode;
  title: string;
  description?: string;
  /** The next action this empty state teaches (kit rule: never a dead end). */
  action?: ReactNode;
  className?: string;
};

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-2 px-6 py-12 text-center',
        className,
      )}
    >
      {icon && (
        <div aria-hidden="true" className="text-ink-300">
          {icon}
        </div>
      )}
      <h3 className="font-display text-lg text-ink-900">{title}</h3>
      {description && <p className="max-w-sm text-sm text-ink-500">{description}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}
