import { useState } from 'react';
import { Button, FieldError, Form, Input, Label, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { SettingsSection } from '~/features/account/components/organisms/settings-section';
import { detectLocale } from '~/i18n/detect-locale';
import { userClient } from '~/lib/api/client';

const INPUT =
  'w-full rounded-xl border border-cream-300 bg-cream-50 px-3.5 py-2.5 text-sm text-ink-900 outline-none transition placeholder:text-ink-300 focus:border-brand-400 focus:ring-2 focus:ring-brand-100';
const BTN =
  'mt-2 w-full rounded-xl bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition data-[hovered]:bg-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

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
        <div className="rounded-xl border border-brand-200 bg-brand-50 px-4 py-3 text-sm text-brand-800">
          {t(
            'account.changeEmail.hint',
            'Your current email stays active until you verify the new one.',
          )}
        </div>
      ) : (
        <Form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <TextField name="newEmail" type="email" isRequired className="flex flex-col gap-1.5">
            <Label className="text-sm font-medium text-ink-700">
              {t('account.changeEmail.newEmail', 'New email')}
            </Label>
            <Input autoComplete="email" className={INPUT} />
            <FieldError className="text-xs text-red-600" />
          </TextField>
          <TextField name="password" type="password" isRequired className="flex flex-col gap-1.5">
            <Label className="text-sm font-medium text-ink-700">
              {t('account.changeEmail.password', 'Current password')}
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
          <Button type="submit" isDisabled={pending} className={BTN}>
            {pending
              ? t('account.changeEmail.submitting', 'Saving…')
              : t('account.changeEmail.submit', 'Change email')}
          </Button>
        </Form>
      )}
    </SettingsSection>
  );
}
