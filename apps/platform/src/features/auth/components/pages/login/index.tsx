import { useNavigate, useParams } from '@tanstack/react-router';
import { Banner, Button, Field } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { authClient } from '~/lib/api/client';
import { useRedirectParam } from '~/lib/redirect';
import { useAuthStore } from '~/store/auth';

export function LoginPage() {
  const { locale } = useParams({ from: '/$locale/auth/login' });
  const navigate = useNavigate();
  const { t } = useTranslation();
  const setAuth = useAuthStore((s) => s.setAuth);
  const { target, raw: redirectRaw } = useRedirectParam();
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setPending(true);
    const form = new FormData(e.currentTarget);
    try {
      const res = await authClient.login({
        email: form.get('email') as string,
        password: form.get('password') as string,
      });
      setAuth(res.accessToken, {
        id: res.user?.id ?? '',
        email: res.user?.email ?? '',
        displayName: res.user?.displayName ?? '',
      });
      // Bounce back to the originally requested page when present; otherwise the
      // account home. A full navigation keeps typed-router happy for arbitrary
      // internal paths and re-bootstraps auth from the persisted token.
      if (target === '/') {
        await navigate({ to: '/' });
      } else {
        window.location.assign(target);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setPending(false);
    }
  }

  const signupHref = redirectRaw
    ? `/${locale}/auth/signup?redirect=${encodeURIComponent(redirectRaw)}`
    : `/${locale}/auth/signup`;

  return (
    <AuthCard
      heading={t('auth.login.title', 'Sign in')}
      description={t('auth.login.description', 'Welcome back to tendersbay.')}
    >
      <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field
          name="email"
          type="email"
          label={t('auth.login.email', 'Email')}
          autoComplete="email"
          isRequired
        />
        <div className="flex flex-col gap-1.5">
          <Field
            name="password"
            type="password"
            label={t('auth.login.password', 'Password')}
            autoComplete="current-password"
            isRequired
          />
          <a
            href={`/${locale}/auth/forgot-password`}
            className="self-end text-xs font-medium text-brand-700 hover:text-brand-800"
          >
            {t('auth.login.forgotPassword', 'Forgot password?')}
          </a>
        </div>
        {error && <Banner tone="error">{error}</Banner>}
        <Button type="submit" isDisabled={pending} className="mt-2 w-full">
          {pending ? t('auth.login.submitting', 'Signing in…') : t('auth.login.submit', 'Sign in')}
        </Button>
      </Form>
      <p className="mt-6 text-center text-sm text-ink-500">
        {t('auth.login.noAccount', "Don't have an account?")}{' '}
        <a href={signupHref} className="font-semibold text-brand-700 hover:text-brand-800">
          {t('auth.login.signUp', 'Sign up')}
        </a>
      </p>
    </AuthCard>
  );
}
