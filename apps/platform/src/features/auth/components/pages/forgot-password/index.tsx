import { useParams } from '@tanstack/react-router';
import { Banner, Button, Field } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { authClient } from '~/lib/api/client';

export function ForgotPasswordPage() {
  const { locale } = useParams({ from: '/$locale/auth/forgot-password' });
  const { t } = useTranslation();
  const [done, setDone] = useState(false);
  const [pending, setPending] = useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setPending(true);
    const form = new FormData(e.currentTarget);
    try {
      await authClient.forgotPassword({ email: form.get('email') as string, locale });
    } catch {
      // Always show success — never reveal whether an address exists.
    } finally {
      setDone(true);
      setPending(false);
    }
  }

  if (done) {
    return (
      <AuthCard
        heading={t('auth.forgot.checkEmail', 'Check your email')}
        description={t(
          'auth.forgot.checkEmailBody',
          "If an account exists for that address, you'll receive a reset link shortly.",
        )}
      >
        <Banner tone="success">
          {t(
            'auth.forgot.checkEmailHint',
            "The link expires in 1 hour. Check your spam folder if you don't see it.",
          )}
        </Banner>
        <p className="mt-4 text-center text-sm text-ink-500">
          <a
            href={`/${locale}/auth/login`}
            className="font-semibold text-brand-700 hover:text-brand-800"
          >
            {t('auth.forgot.backToLogin', 'Back to sign in')}
          </a>
        </p>
      </AuthCard>
    );
  }

  return (
    <AuthCard
      heading={t('auth.forgot.title', 'Reset password')}
      description={t(
        'auth.forgot.description',
        "Enter your email and we'll send you a reset link.",
      )}
    >
      <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field
          name="email"
          type="email"
          label={t('auth.forgot.email', 'Email')}
          autoComplete="email"
          isRequired
        />
        <Button type="submit" isDisabled={pending} className="mt-2 w-full">
          {pending
            ? t('auth.forgot.submitting', 'Sending…')
            : t('auth.forgot.submit', 'Send reset link')}
        </Button>
      </Form>
      <p className="mt-6 text-center text-sm text-ink-500">
        <a
          href={`/${locale}/auth/login`}
          className="font-semibold text-brand-700 hover:text-brand-800"
        >
          {t('auth.forgot.backToLogin', 'Back to sign in')}
        </a>
      </p>
    </AuthCard>
  );
}
