import { Link } from '@tanstack/react-router';
import { Fragment } from 'react';

export interface Crumb {
  label: string;
  to?: string;
  params?: Record<string, string>;
}

const CRUMB = 'text-ink-500 no-underline transition-colors hover:text-ink-900';
const CURRENT = 'text-ink-900 font-medium';

export function Breadcrumb({ items }: { items: Crumb[] }) {
  return (
    <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-1.5 text-sm">
      {items.map((c, i) => {
        const last = i === items.length - 1;
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: crumbs are a static, non-reorderable list
          <Fragment key={`${c.label}-${i}`}>
            {c.to && !last ? (
              <Link to={c.to} params={c.params} className={CRUMB}>
                {c.label}
              </Link>
            ) : (
              <span className={last ? CURRENT : CRUMB}>{c.label}</span>
            )}
            {!last && (
              <span className="text-ink-300" aria-hidden="true">
                /
              </span>
            )}
          </Fragment>
        );
      })}
    </nav>
  );
}
