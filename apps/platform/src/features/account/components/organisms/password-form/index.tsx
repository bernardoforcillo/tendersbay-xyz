import { Banner, Button, Field } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { SettingsSection } from '~/features/account/components/organisms/settings-section';
import { userClient } from '~/lib/api/client';

export function PasswordForm() {
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

  return (
    <SettingsSection
      title={
        done
          ? t('account.changePassword.successTitle', 'Password updated')
          : t('account.changePassword.title', 'Change password')
      }
      description={
        done
          ? t(
              'account.changePassword.successBody',
              'Your password has been changed. Sign in again on other devices.',
            )
          : t('account.changePassword.description', 'New password must be at least 12 characters.')
      }
    >
      {done ? (
        <Banner tone="success">
          {t(
            'account.changePassword.successHint',
            'Other active sessions remain valid until they expire.',
          )}
        </Banner>
      ) : (
        <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Field
            name="currentPassword"
            type="password"
            isRequired
            autoComplete="current-password"
            label={t('account.changePassword.current', 'Current password')}
          />
          <Field
            name="newPassword"
            type="password"
            isRequired
            minLength={12}
            autoComplete="new-password"
            label={t('account.changePassword.new', 'New password')}
          />
          {error && <Banner tone="error">{error}</Banner>}
          <Button type="submit" isDisabled={pending}>
            {pending
              ? t('account.changePassword.submitting', 'Saving…')
              : t('account.changePassword.submit', 'Change password')}
          </Button>
        </Form>
      )}
    </SettingsSection>
  );
}
