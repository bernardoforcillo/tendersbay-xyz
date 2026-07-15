import type { ReactNode } from 'react';
import { cn } from '../../../cn';

export type PageHeaderProps = {
  /** Element before the title (e.g. the app's sidebar toggle or a monogram tile). */
  leading?: ReactNode;
  /** Main heading. Omit for an actions-only bar. */
  title?: ReactNode;
  /** Secondary line under the title. */
  subtitle?: ReactNode;
  /** Right-aligned actions. */
  actions?: ReactNode;
  /** Content below the header row (e.g. a tab nav). */
  children?: ReactNode;
  className?: string;
};

export function PageHeader({
  leading,
  title,
  subtitle,
  actions,
  children,
  className,
}: PageHeaderProps) {
  return (
    <header
      className={cn(
        'flex flex-col gap-4 border-b border-cream-200 bg-white px-4 py-3 lg:px-6 lg:py-4',
        className,
      )}
    >
      <div className="flex items-center gap-3">
        {leading}
        <div className="min-w-0 flex-1">
          {title && <h1 className="truncate font-display text-2xl text-ink-900">{title}</h1>}
          {subtitle && <p className="truncate text-sm text-ink-500">{subtitle}</p>}
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
      </div>
      {children}
    </header>
  );
}
