import { Link } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '~/store/auth';

/**
 * Returns a function that wraps `children` in a link to a tender's detail page,
 * choosing the authenticated (`/explore/tenders/$id`) or public
 * (`/$locale/tenders/$id`) route by auth state — so the same call site links
 * correctly for logged-in and anonymous users, and an anon user never lands on
 * the auth-guarded route.
 */
export function useTenderLink(): (
  id: string,
  children: ReactNode,
  className?: string,
) => ReactNode {
  const authed = useAuthStore((s) => s.isAuthenticated);
  const { i18n } = useTranslation();
  return (id, children, className) =>
    authed ? (
      <Link to="/explore/tenders/$id" params={{ id }} className={className}>
        {children}
      </Link>
    ) : (
      <Link to="/$locale/tenders/$id" params={{ locale: i18n.language, id }} className={className}>
        {children}
      </Link>
    );
}
