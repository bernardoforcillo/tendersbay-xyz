import { ChatWindow, PageHeader } from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';

export function AccountExplorePage() {
  return (
    <AccountLayout>
      <PageHeader />
      <div className="flex min-h-full flex-1 flex-col px-4 pb-16">
        <ChatWindow />
      </div>
    </AccountLayout>
  );
}
