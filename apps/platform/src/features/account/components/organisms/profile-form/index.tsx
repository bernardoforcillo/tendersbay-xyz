import { Banner, Button, Field } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { SettingsSection } from '~/features/account/components/organisms/settings-section';
import { userClient } from '~/lib/api/client';
import { type AuthUser, useAuthStore } from '~/store/auth';

export function ProfileForm() {
  const { t } = useTranslation();
  const { user, setAuth, accessToken } = useAuthStore();
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [saved, setSaved] = useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setSaved(false);
    setPending(true);
    const form = new FormData(e.currentTarget);
    try {
      const res = await userClient.updateProfile({
        displayName: form.get('displayName') as string,
      });
      const updated: AuthUser = {
        id: res.user?.id ?? '',
        email: res.user?.email ?? '',
        displayName: res.user?.displayName ?? '',
      };
      setAuth(accessToken ?? '', updated);
      setSaved(true);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Update failed');
    } finally {
      setPending(false);
    }
  }

  return (
    <SettingsSection
      title={t('account.profile.title', 'Profile')}
      description={t(
        'account.profile.description',
        'Update your display name and how others see you.',
      )}
    >
      <div className="mb-5 flex items-center gap-4">
        <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-brand-100 text-lg font-bold text-brand-700">
          {(user?.displayName?.[0] ?? user?.email?.[0] ?? '?').toUpperCase()}
        </div>
        <div>
          <p className="text-sm font-semibold text-ink-900">{user?.displayName}</p>
          <p className="text-sm text-ink-500">{user?.email}</p>
        </div>
      </div>
      <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field
          name="displayName"
          defaultValue={user?.displayName}
          isRequired
          autoComplete="name"
          label={t('account.profile.displayName', 'Display name')}
        />
        {error && <Banner tone="error">{error}</Banner>}
        {saved && <Banner tone="success">{t('account.profile.saved', 'Saved!')}</Banner>}
        <Button type="submit" isDisabled={pending}>
          {pending
            ? t('account.profile.submitting', 'Saving…')
            : t('account.profile.submit', 'Save changes')}
        </Button>
      </Form>
    </SettingsSection>
  );
}
