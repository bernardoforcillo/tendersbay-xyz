import { Banner, Button, Field } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { SettingsSection } from '~/features/account/components/organisms/settings-section';
import { detectLocale } from '~/i18n/detect-locale';
import { userClient } from '~/lib/api/client';

export function EmailForm() {
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
      await userClient.changeEmail({
        newEmail: form.get('newEmail') as string,
        password: form.get('password') as string,
        locale: detectLocale(),
      });
      setDone(true);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to change email');
    } finally {
      setPending(false);
    }
  }

  return (
    <SettingsSection
      title={
        done
          ? t('account.changeEmail.checkEmail', 'Check your new email')
          : t('account.changeEmail.title', 'Change email')
      }
      description={
        done
          ? t(
              'account.changeEmail.prompt',
              'We sent a verification link to your new address. Click it to confirm the change.',
            )
          : t(
              'account.changeEmail.description',
              "We'll send a verification link to the new address.",
            )
      }
    >
      {done ? (
        <Banner tone="success">
          {t(
            'account.changeEmail.hint',
            'Your current email stays active until you verify the new one.',
          )}
        </Banner>
      ) : (
        <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Field
            name="newEmail"
            type="email"
            isRequired
            autoComplete="email"
            label={t('account.changeEmail.newEmail', 'New email')}
          />
          <Field
            name="password"
            type="password"
            isRequired
            autoComplete="current-password"
            label={t('account.changeEmail.password', 'Current password')}
          />
          {error && <Banner tone="error">{error}</Banner>}
          <Button type="submit" isDisabled={pending}>
            {pending
              ? t('account.changeEmail.submitting', 'Saving…')
              : t('account.changeEmail.submit', 'Change email')}
          </Button>
        </Form>
      )}
    </SettingsSection>
  );
}
