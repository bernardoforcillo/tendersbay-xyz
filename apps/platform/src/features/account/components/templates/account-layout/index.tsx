import { Link, useNavigate } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Logo } from '~/features/landing/components/atoms';
import { detectLocale } from '~/i18n/detect-locale';
import { authClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

type AccountLayoutProps = {
  title: string;
  description?: string;
  children: ReactNode;
};

const NAV_LINK =
  'rounded-lg px-3 py-1.5 text-sm font-medium text-ink-600 no-underline transition hover:bg-ink-50 hover:text-ink-900 [&[aria-current=page]]:bg-brand-50 [&[aria-current=page]]:text-brand-700';

export function AccountLayout({ title, description, children }: AccountLayoutProps) {
  const { i18n } = useTranslation();
  const navigate = useNavigate();
  const clearAuth = useAuthStore((s) => s.clearAuth);

  async function handleLogout() {
    try {
      await authClient.logout({});
    } catch {
      // best-effort
    }
    clearAuth();
    await navigate({ to: '/$locale/auth/login', params: { locale: detectLocale() } });
  }

  return (
    <div className="min-h-screen bg-cream-100">
      <header className="sticky top-0 z-40 border-b border-cream-200 bg-white/80 backdrop-blur-md">
        <div className="mx-auto flex max-w-4xl items-center justify-between px-6 py-3">
          <Link
            to="/$locale"
            params={{ locale: i18n.language }}
            aria-label="tendersbay home"
            className="no-underline outline-none"
          >
            <Logo />
          </Link>
          <nav className="hidden items-center gap-0.5 sm:flex">
            <Link to="/account/profile" className={NAV_LINK}>
              Profile
            </Link>
            <Link to="/account/change-email" className={NAV_LINK}>
              Email
            </Link>
            <Link to="/account/change-password" className={NAV_LINK}>
              Password
            </Link>
          </nav>
          <Button
            onPress={handleLogout}
            className="rounded-full border border-cream-300 px-3.5 py-1.5 text-sm font-medium text-ink-700 outline-none transition data-[hovered]:border-cream-400 data-[hovered]:bg-cream-50 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600"
          >
            Log out
          </Button>
        </div>
      </header>
      <main className="mx-auto max-w-lg px-4 py-12">
        <div className="mb-6">
          <h1 className="font-display text-2xl leading-tight text-ink-900">{title}</h1>
          {description && (
            <p className="mt-1.5 text-sm leading-relaxed text-ink-500">{description}</p>
          )}
        </div>
        <div className="rounded-2xl border border-cream-200 bg-white px-8 py-8 shadow-soft-md">
          {children}
        </div>
      </main>
    </div>
  );
}
