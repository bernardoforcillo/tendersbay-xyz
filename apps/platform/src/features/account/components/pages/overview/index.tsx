import { SearchDock } from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';

export function AccountOverviewPage() {
  return (
    <AccountLayout>
      {/* Dock pinned to the bottom — animates to center when navigating to Explore */}
      <div className="flex min-h-full flex-col">
        <div className="mt-auto flex justify-center px-4 pb-6 pt-4">
          <SearchDock />
        </div>
      </div>
    </AccountLayout>
  );
}
