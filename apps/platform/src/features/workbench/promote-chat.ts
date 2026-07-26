import { agentClient } from '~/lib/api/client';
import { useChatStore } from '~/store/chat';

/**
 * Promote an explore (workspace-level) chat into a workbench so the one
 * continuous assistant carries over. Promotion is ONE-WAY: the backend's
 * UpdateSession only assigns workbench_id when it is non-empty
 * (services/backend/internal/adapter/postgres/chat_repo.go:66), so a promoted
 * chat can never be demoted back to workspace scope. Rebinds the shared chat
 * store to the promoted chat under the target workbench.
 */
export async function promoteChatToWorkbench(chatId: string, workbenchId: string): Promise<void> {
  await agentClient.updateChat({ chatId, workbenchId });
  const store = useChatStore.getState();
  store.setCurrentChat(chatId);
  store.setCurrentWorkbench(workbenchId);
}
