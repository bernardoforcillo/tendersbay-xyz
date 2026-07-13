import { Link } from '@tanstack/react-router';
import { Card } from '@tendersbay/components/core';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Logo } from '~/features/landing/components/atoms';

type AuthCardProps = {
  heading: string;
  description?: string;
  children?: ReactNode;
};

export function AuthCard({ heading, description, children }: AuthCardProps) {
  const { i18n } = useTranslation();

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-cream-100 px-4 py-12">
      <div className="w-full max-w-sm">
        <Link
          to="/$locale"
          params={{ locale: i18n.language }}
          aria-label="tendersbay home"
          className="mb-8 flex justify-center no-underline outline-none"
        >
          <Logo />
        </Link>
        <Card className="border border-cream-200 px-8 py-8 shadow-soft-md">
          <h1 className="font-display text-3xl leading-tight text-ink-900">{heading}</h1>
          {description && (
            <p className="mt-1.5 text-sm leading-relaxed text-ink-500">{description}</p>
          )}
          <div className="mt-6">{children}</div>
        </Card>
      </div>
    </div>
  );
}
