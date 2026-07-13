import { useParams } from '@tanstack/react-router';
import { Banner, Button, Field } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { authClient } from '~/lib/api/client';
import { useRedirectParam } from '~/lib/redirect';

export function SignupPage() {
  const { locale } = useParams({ from: '/$locale/auth/signup' });
  const { t } = useTranslation();
  const { raw: redirectRaw } = useRedirectParam();
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [pending, setPending] = useState(false);

  const loginHref = redirectRaw
    ? `/${locale}/auth/login?redirect=${encodeURIComponent(redirectRaw)}`
    : `/${locale}/auth/login`;

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setPending(true);
    const form = new FormData(e.currentTarget);
    try {
      await authClient.signUp({
        email: form.get('email') as string,
        password: form.get('password') as string,
        displayName: form.get('displayName') as string,
        locale,
      });
      setDone(true);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Sign-up failed');
    } finally {
      setPending(false);
    }
  }

  if (done) {
    return (
      <AuthCard
        heading={t('auth.signup.checkEmail', 'Check your email')}
        description={t(
          'auth.signup.verifyPrompt',
          'We sent you a verification link. Click it to activate your account.',
        )}
      >
        <Banner tone="success">
          {t(
            'auth.signup.checkEmailHint',
            "The link expires in 24 hours. Check your spam folder if you don't see it.",
          )}
        </Banner>
        <p className="mt-4 text-center text-sm text-ink-500">
          <a href={loginHref} className="font-semibold text-brand-700 hover:text-brand-800">
            {t('auth.signup.backToLogin', 'Back to sign in')}
          </a>
        </p>
      </AuthCard>
    );
  }

  return (
    <AuthCard
      heading={t('auth.signup.title', 'Create account')}
      description={t('auth.signup.description', 'Start winning EU tenders with AI agents.')}
    >
      <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field
          name="displayName"
          label={t('auth.signup.displayName', 'Name')}
          autoComplete="name"
          isRequired
        />
        <Field
          name="email"
          type="email"
          label={t('auth.signup.email', 'Email')}
          autoComplete="email"
          isRequired
        />
        <Field
          name="password"
          type="password"
          label={t('auth.signup.password', 'Password')}
          autoComplete="new-password"
          isRequired
        />
        {error && <Banner tone="error">{error}</Banner>}
        <Button type="submit" isDisabled={pending} className="mt-2 w-full">
          {pending
            ? t('auth.signup.submitting', 'Creating…')
            : t('auth.signup.submit', 'Create account')}
        </Button>
      </Form>
      <p className="mt-6 text-center text-sm text-ink-500">
        {t('auth.signup.login', 'Already have an account?')}{' '}
        <a href={loginHref} className="font-semibold text-brand-700 hover:text-brand-800">
          {t('auth.signup.signIn', 'Sign in')}
        </a>
      </p>
    </AuthCard>
  );
}
