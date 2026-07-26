import { usePostHog } from 'posthog-js/react';
import { useEffect, useRef } from 'react';
import { ChatWindow } from '~/features/account/components/organisms';
import { useWorkbenchContext } from '~/features/workbench/context';
import { agentClient } from '~/lib/api/client';
import { useChatStore } from '~/store/chat';

/**
 * The one continuous assistant, scoped to a workbench. On entering a workbench
 * whose id differs from the store's currentWorkbenchId, it resets the shared chat
 * surface, then resumes this workbench's most-recently-updated conversation — or
 * leaves it empty for a lazy create on first send (ChatWindow binds the new chat
 * to this workbench via its `workbenchId` prop). ListChats is workspace-scoped and
 * returns every chat's `workbenchId`, so the workbench filter is applied
 * client-side (no proto change; deferred until a chat-volume trigger).
 */
export function WorkbenchChatPanel() {
  const posthog = usePostHog();
  const { workbenchId, workbench } = useWorkbenchContext();
  const workspaceId = workbench.workspaceId;
  const enteredRef = useRef<string | null>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: posthog is stable
  useEffect(() => {
    if (enteredRef.current === workbenchId) return;
    enteredRef.current = workbenchId;

    const store = useChatStore.getState();
    if (store.currentWorkbenchId !== workbenchId) {
      store.reset();
      store.setCurrentWorkbench(workbenchId);
    }

    let active = true;
    agentClient
      .listChats({ workspaceId })
      .then((res) => {
        if (!active) return;
        const forWorkbench = res.chats
          .filter((c) => c.workbenchId === workbenchId)
          .sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : -1));
        const mostRecent = forWorkbench[0];
        if (mostRecent) {
          useChatStore.getState().setCurrentChat(mostRecent.id);
          posthog?.capture('workbench_chat_continued', { location: 'workbench' });
        }
      })
      .catch(() => {});

    return () => {
      active = false;
    };
  }, [workbenchId, workspaceId]);

  return <ChatWindow location="workbench" workbenchId={workbenchId} />;
}
