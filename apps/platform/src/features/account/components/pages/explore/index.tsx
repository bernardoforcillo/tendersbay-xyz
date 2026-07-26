import { useEffect } from 'react';
import { ChatWindow, PageHeader } from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { useChatStore } from '~/store/chat';

export function AccountExplorePage() {
  // Symmetric counterpart to WorkbenchChatPanel's entry guard
  // (~/features/workbench/components/organisms/workbench-chat-panel): the
  // explore surface is always workspace-scoped (no workbenchId). currentChatId
  // + currentWorkbenchId persist to sessionStorage, so navigating explore ->
  // workbench -> explore would otherwise leave the store scoped to the
  // workbench, and the explore ChatWindow below would hydrate that
  // workbench-scoped conversation into /explore. Reset before it mounts.
  // Checking store state directly (not a reactive selector) and re-running
  // this on every mount keeps it idempotent, so a StrictMode double-invoke is
  // a no-op rather than needing an `enteredRef`-style once-guard.
  useEffect(() => {
    const store = useChatStore.getState();
    if (store.currentWorkbenchId !== null) {
      store.reset();
      store.setCurrentWorkbench(null);
    }
  }, []);

  return (
    <AccountLayout>
      <PageHeader />
      <div className="flex min-h-full flex-1 flex-col px-4 pb-16">
        <ChatWindow />
      </div>
    </AccountLayout>
  );
}
