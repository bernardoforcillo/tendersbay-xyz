import { useNavigate } from '@tanstack/react-router';
import { ChevronLeft } from 'lucide-react';
import { Button } from 'react-aria-components';
import {
  DeleteForm,
  EmailForm,
  PasswordForm,
  ProfileForm,
} from '~/features/account/components/organisms';

export function AccountSettingsPage() {
  const navigate = useNavigate();

  return (
    <div className="min-h-screen bg-cream-50">
      <header className="sticky top-0 z-40 border-b border-cream-200 bg-white">
        <div className="mx-auto flex h-14 max-w-3xl items-center gap-3 px-4 sm:px-6">
          <Button
            onPress={() => navigate({ to: '/' })}
            className="flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-ink-500 outline-none transition data-[hovered]:bg-cream-100 data-[hovered]:text-ink-900 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600"
          >
            <ChevronLeft size={16} aria-hidden="true" />
            Home
          </Button>
          <h1 className="text-sm font-semibold text-ink-900">Account settings</h1>
        </div>
      </header>

      <div className="mx-auto max-w-3xl divide-y divide-cream-200 px-4 sm:px-6">
        <ProfileForm />
        <EmailForm />
        <PasswordForm />
        <DeleteForm />
      </div>
    </div>
  );
}
