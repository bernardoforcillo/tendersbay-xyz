import { useTranslation } from 'react-i18next';
import {
  DeleteForm,
  EmailForm,
  PageHeader,
  PasswordForm,
  ProfileForm,
} from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';

export function AccountSettingsPage() {
  const { t } = useTranslation();
  return (
    <AccountLayout>
      <PageHeader title={t('account.settings.title', 'Account settings')} />
      <div className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-6">
        <div className="divide-y divide-cream-200">
          <ProfileForm />
          <EmailForm />
          <PasswordForm />
          <DeleteForm />
        </div>
      </div>
    </AccountLayout>
  );
}
