import { useNavigate } from '@tanstack/react-router';
import { Banner, Button, Field } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { SettingsSection } from '~/features/account/components/organisms/settings-section';
import { detectLocale } from '~/i18n/detect-locale';
import { authClient, userClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

export function DeleteForm() {
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
    <SettingsSection
      title={t('account.delete.title', 'Delete account')}
      description={t(
        'account.delete.description',
        'This action is permanent and cannot be undone. All your data, workspaces, and settings will be permanently deleted.',
      )}
      variant="danger"
    >
      <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field
          name="password"
          type="password"
          isRequired
          autoComplete="current-password"
          label={t('account.delete.password', 'Confirm with your password')}
        />
        {error && <Banner tone="error">{error}</Banner>}
        <Button type="submit" isDisabled={pending} variant="danger">
          {pending
            ? t('account.delete.submitting', 'Deleting…')
            : t('account.delete.submit', 'Delete my account')}
        </Button>
      </Form>
    </SettingsSection>
  );
}
