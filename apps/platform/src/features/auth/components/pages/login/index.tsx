import { useNavigate, useParams } from '@tanstack/react-router';
import { useState } from 'react';
import { Button, Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Field } from '~/features/auth/components/atoms/field';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { authClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

const BTN =
  'mt-2 w-full rounded-xl bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition data-[hovered]:bg-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

export function LoginPage() {
  const { locale } = useParams({ from: '/$locale/auth/login' });
  const navigate = useNavigate();
  const { t } = useTranslation();
  const setAuth = useAuthStore((s) => s.setAuth);
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
      await navigate({ to: '/account/profile' });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setPending(false);
    }
  }

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
        {error && (
          <p
            role="alert"
            className="rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700"
          >
            {error}
          </p>
        )}
        <Button type="submit" isDisabled={pending} className={BTN}>
          {pending ? t('auth.login.submitting', 'Signing in…') : t('auth.login.submit', 'Sign in')}
        </Button>
      </Form>
      <p className="mt-6 text-center text-sm text-ink-500">
        {t('auth.login.noAccount', "Don't have an account?")}{' '}
        <a
          href={`/${locale}/auth/signup`}
          className="font-semibold text-brand-700 hover:text-brand-800"
        >
          {t('auth.login.signUp', 'Sign up')}
        </a>
      </p>
    </AuthCard>
  );
}
