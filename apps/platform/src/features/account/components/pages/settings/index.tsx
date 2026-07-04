import {
  DeleteForm,
  EmailForm,
  PasswordForm,
  ProfileForm,
} from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';

export function AccountSettingsPage() {
  return (
    <AccountLayout>
      <div className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-6">
        <h1 className="mb-6 text-lg font-semibold text-ink-900">Account settings</h1>
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
