import { useNavigate, useSearch } from '@tanstack/react-router';
import { Banner } from '@tendersbay/components/core';
import { CheckCircle2, Loader2 } from 'lucide-react';
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
        <Banner tone="error">
          {t('auth.verify.error', 'This link is invalid or has expired. Request a new one.')}
        </Banner>
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
          <CheckCircle2 className="h-12 w-12 text-brand-500" strokeWidth={1.5} aria-hidden="true" />
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
        <Loader2 className="h-8 w-8 animate-spin text-brand-500" aria-hidden="true" />
      </div>
    </AuthCard>
  );
}
