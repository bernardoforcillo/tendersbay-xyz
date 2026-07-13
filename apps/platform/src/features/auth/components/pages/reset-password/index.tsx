import { useNavigate, useParams, useSearch } from '@tanstack/react-router';
import { Banner, Button, Field } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { authClient } from '~/lib/api/client';

export function ResetPasswordPage() {
  const { token } = useSearch({ from: '/$locale/auth/reset-password' }) as { token?: string };
  const { locale } = useParams({ from: '/$locale/auth/reset-password' });
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!token) return;
    setError(null);
    setPending(true);
    const form = new FormData(e.currentTarget);
    try {
      await authClient.resetPassword({
        token,
        newPassword: form.get('password') as string,
      });
      await navigate({ to: '/$locale/auth/login', params: { locale } });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Reset failed');
    } finally {
      setPending(false);
    }
  }

  if (!token) {
    return (
      <AuthCard heading={t('auth.reset.invalidTitle', 'Invalid link')}>
        <Banner tone="error">
          {t('auth.reset.invalid', 'This reset link is missing or has expired.')}
        </Banner>
        <p className="mt-4 text-center text-sm text-ink-500">
          <a
            href={`/${locale}/auth/forgot-password`}
            className="font-semibold text-brand-700 hover:text-brand-800"
          >
            {t('auth.reset.requestNew', 'Request a new link')}
          </a>
        </p>
      </AuthCard>
    );
  }

  return (
    <AuthCard
      heading={t('auth.reset.title', 'Set new password')}
      description={t(
        'auth.reset.description',
        'Choose a strong password of at least 12 characters.',
      )}
    >
      <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field
          name="password"
          type="password"
          label={t('auth.reset.password', 'New password')}
          autoComplete="new-password"
          isRequired
        />
        {error && <Banner tone="error">{error}</Banner>}
        <Button type="submit" isDisabled={pending} className="mt-2 w-full">
          {pending ? t('auth.reset.submitting', 'Saving…') : t('auth.reset.submit', 'Set password')}
        </Button>
      </Form>
    </AuthCard>
  );
}
