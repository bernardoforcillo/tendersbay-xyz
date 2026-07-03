import { useState } from 'react';
import { Button, FieldError, Form, Input, Label, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { SettingsSection } from '~/features/account/components/organisms/settings-section';
import { userClient } from '~/lib/api/client';
import { type AuthUser, useAuthStore } from '~/store/auth';

const INPUT =
  'w-full rounded-xl border border-cream-300 bg-cream-50 px-3.5 py-2.5 text-sm text-ink-900 outline-none transition placeholder:text-ink-300 focus:border-brand-400 focus:ring-2 focus:ring-brand-100';
const BTN =
  'mt-2 w-full rounded-xl bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition data-[hovered]:bg-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

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
        <TextField
          name="displayName"
          defaultValue={user?.displayName}
          isRequired
          className="flex flex-col gap-1.5"
        >
          <Label className="text-sm font-medium text-ink-700">
            {t('account.profile.displayName', 'Display name')}
          </Label>
          <Input autoComplete="name" className={INPUT} />
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
        {saved && (
          <p className="rounded-lg border border-brand-200 bg-brand-50 px-3 py-2.5 text-sm text-brand-700">
            {t('account.profile.saved', 'Saved!')}
          </p>
        )}
        <Button type="submit" isDisabled={pending} className={BTN}>
          {pending
            ? t('account.profile.submitting', 'Saving…')
            : t('account.profile.submit', 'Save changes')}
        </Button>
      </Form>
    </SettingsSection>
  );
}
