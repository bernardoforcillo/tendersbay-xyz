import { useState } from 'react';
import { Button, FieldError, Form, Input, Label, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { userClient } from '~/lib/api/client';

const INPUT =
  'w-full rounded-xl border border-cream-300 bg-cream-50 px-3.5 py-2.5 text-sm text-ink-900 outline-none transition placeholder:text-ink-300 focus:border-brand-400 focus:ring-2 focus:ring-brand-100';
const BTN =
  'mt-2 w-full rounded-xl bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition data-[hovered]:bg-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

export function ChangePasswordPage() {
  const { t } = useTranslation();
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [pending, setPending] = useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setPending(true);
    const form = new FormData(e.currentTarget);
    try {
      await userClient.changePassword({
        currentPassword: form.get('currentPassword') as string,
        newPassword: form.get('newPassword') as string,
      });
      setDone(true);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to change password');
    } finally {
      setPending(false);
    }
  }

  if (done) {
    return (
      <AccountLayout
        title={t('account.changePassword.successTitle', 'Password updated')}
        description={t(
          'account.changePassword.successBody',
          'Your password has been changed. Sign in again on other devices.',
        )}
      >
        <div className="rounded-xl border border-brand-200 bg-brand-50 px-4 py-3 text-sm text-brand-800">
          {t(
            'account.changePassword.successHint',
            'Other active sessions remain valid until they expire.',
          )}
        </div>
      </AccountLayout>
    );
  }

  return (
    <AccountLayout
      title={t('account.changePassword.title', 'Change password')}
      description={t(
        'account.changePassword.description',
        'New password must be at least 12 characters.',
      )}
    >
      <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <TextField
          name="currentPassword"
          type="password"
          isRequired
          className="flex flex-col gap-1.5"
        >
          <Label className="text-sm font-medium text-ink-700">
            {t('account.changePassword.current', 'Current password')}
          </Label>
          <Input autoComplete="current-password" className={INPUT} />
          <FieldError className="text-xs text-red-600" />
        </TextField>
        <TextField
          name="newPassword"
          type="password"
          isRequired
          minLength={12}
          className="flex flex-col gap-1.5"
        >
          <Label className="text-sm font-medium text-ink-700">
            {t('account.changePassword.new', 'New password')}
          </Label>
          <Input autoComplete="new-password" className={INPUT} />
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
        <Button type="submit" isDisabled={pending} className={BTN}>
          {pending
            ? t('account.changePassword.submitting', 'Saving…')
            : t('account.changePassword.submit', 'Change password')}
        </Button>
      </Form>
    </AccountLayout>
  );
}
