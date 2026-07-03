import { useNavigate, useSearch } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { authClient } from '~/lib/api/client';

export function VerifyEmailPage() {
  const { token, type } = useSearch({ from: '/$locale/auth/verify-email' }) as {
    token?: string;
    type?: string;
  };
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  // Prevent React strict-mode double-invocation: the first call deletes the
  // token from the DB; a second call immediately after would fail with "invalid".
  const called = useRef(false);

  useEffect(() => {
    if (!token || !type) {
      setStatus('error');
      return;
    }
    if (called.current) return;
    called.current = true;
    authClient
      .verifyEmail({ token, type })
      .then(async () => {
        setStatus('success');
        await new Promise((r) => setTimeout(r, 2000));
        await navigate({ to: '/' });
      })
      .catch(() => setStatus('error'));
  }, [token, type, navigate]);

  if (status === 'error') {
    return (
      <AuthCard heading={t('auth.verify.errorTitle', 'Link expired')}>
        <div
          role="alert"
          className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
        >
          {t('auth.verify.error', 'This link is invalid or has expired. Request a new one.')}
        </div>
      </AuthCard>
    );
  }

  if (status === 'success') {
    return (
      <AuthCard
        heading={t('auth.verify.successTitle', 'Email verified')}
        description={t('auth.verify.success', "You're all set. Redirecting you now…")}
      >
        <div className="flex justify-center py-4">
          <svg
            className="h-12 w-12 text-brand-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
        </div>
      </AuthCard>
    );
  }

  return (
    <AuthCard
      heading={t('auth.verify.loadingTitle', 'Verifying your email…')}
      description={t('auth.verify.loading', 'Just a moment.')}
    >
      <div className="flex justify-center py-4">
        <svg
          className="h-8 w-8 animate-spin text-brand-500"
          fill="none"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
          />
        </svg>
      </div>
    </AuthCard>
  );
}
