import { beforeEach, describe, expect, it, vi } from 'vitest';
import { agentClient } from '~/lib/api/client';
import { useChatStore } from '~/store/chat';

vi.mock('~/lib/api/client', () => ({ agentClient: { updateChat: vi.fn() } }));

import { promoteChatToWorkbench } from './promote-chat';

describe('promoteChatToWorkbench', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useChatStore.getState().reset();
  });

  it('calls UpdateChat with the target workbench and rebinds the chat store', async () => {
    vi.mocked(agentClient.updateChat).mockResolvedValue({} as never);

    await promoteChatToWorkbench('chat-9', 'wb-1');

    expect(agentClient.updateChat).toHaveBeenCalledWith({ chatId: 'chat-9', workbenchId: 'wb-1' });
    expect(useChatStore.getState().currentChatId).toBe('chat-9');
    expect(useChatStore.getState().currentWorkbenchId).toBe('wb-1');
  });
});
