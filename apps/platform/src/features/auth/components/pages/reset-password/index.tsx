import { useNavigate, useParams, useSearch } from '@tanstack/react-router';
import { useState } from 'react';
import { Button, Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Field } from '~/features/auth/components/atoms/field';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { authClient } from '~/lib/api/client';

const BTN =
  'mt-2 w-full rounded-xl bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition data-[hovered]:bg-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

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
        <div
          role="alert"
          className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
        >
          {t('auth.reset.invalid', 'This reset link is missing or has expired.')}
        </div>
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
        {error && (
          <p
            role="alert"
            className="rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700"
          >
            {error}
          </p>
        )}
        <Button type="submit" isDisabled={pending} className={BTN}>
          {pending ? t('auth.reset.submitting', 'Saving…') : t('auth.reset.submit', 'Set password')}
        </Button>
      </Form>
    </AuthCard>
  );
}
