import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { Button, FieldError, Form, Input, Label, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { detectLocale } from '~/i18n/detect-locale';
import { authClient, userClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

const INPUT =
  'w-full rounded-xl border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-ink-900 outline-none transition placeholder:text-ink-300 focus:border-red-400 focus:ring-2 focus:ring-red-100';
const BTN_DANGER =
  'mt-2 w-full rounded-xl bg-red-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition data-[hovered]:bg-red-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-600 focus-visible:ring-offset-2 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

export function DeleteAccountPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const clearAuth = useAuthStore((s) => s.clearAuth);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setPending(true);
    const form = new FormData(e.currentTarget);
    try {
      await userClient.deleteAccount({ password: form.get('password') as string });
      try {
        await authClient.logout({});
      } catch {
        // best-effort
      }
      clearAuth();
      await navigate({ to: '/$locale/auth/login', params: { locale: detectLocale() } });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Delete failed');
      setPending(false);
    }
  }

  return (
    <AccountLayout
      title={t('account.delete.title', 'Delete account')}
      description={t(
        'account.delete.description',
        'This action is permanent and cannot be undone.',
      )}
    >
      <div className="mb-6 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        {t(
          'account.delete.warning',
          'All your data, workspaces, and settings will be permanently deleted.',
        )}
      </div>
      <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <TextField name="password" type="password" isRequired className="flex flex-col gap-1.5">
          <Label className="text-sm font-medium text-ink-700">
            {t('account.delete.password', 'Confirm with your password')}
          </Label>
          <Input autoComplete="current-password" className={INPUT} />
          <FieldError className="text-xs text-red-600" />
        </TextField>
        {error && (
          <p
            role="alert"
            className="rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700"
          >
            {error}
          </p>
        )}
        <Button type="submit" isDisabled={pending} className={BTN_DANGER}>
          {pending
            ? t('account.delete.submitting', 'Deleting…')
            : t('account.delete.submit', 'Delete my account')}
        </Button>
      </Form>
    </AccountLayout>
  );
}
