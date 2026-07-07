import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  createdAt: string;
}

interface ChatStore {
  currentChatId: string | null;
  messages: ChatMessage[];
  streaming: boolean;
  streamingContent: string;
  credits: {
    remaining: number;
    monthlyMax: number;
    used: number;
    resetDate: string;
  } | null;
  setCurrentChat: (id: string | null) => void;
  addMessage: (msg: ChatMessage) => void;
  setStreaming: (v: boolean) => void;
  appendStreamToken: (token: string) => void;
  setStreamingContent: (content: string) => void;
  setCredits: (credits: ChatStore['credits']) => void;
  reset: () => void;
}

export const useChatStore = create<ChatStore>()(
  persist(
    (set) => ({
      currentChatId: null,
      messages: [],
      streaming: false,
      streamingContent: '',
      credits: null,
      setCurrentChat: (id) => set({ currentChatId: id }),
      addMessage: (msg) => set((s) => ({ messages: [...s.messages, msg] })),
      setStreaming: (v) => set({ streaming: v }),
      appendStreamToken: (token) => set((s) => ({ streamingContent: s.streamingContent + token })),
      setStreamingContent: (content) => set({ streamingContent: content }),
      setCredits: (credits) => set({ credits }),
      reset: () =>
        set({
          messages: [],
          streaming: false,
          streamingContent: '',
          currentChatId: null,
        }),
    }),
    {
      name: 'chat',
      storage: createJSONStorage(() => sessionStorage),
      partialize: (s) => ({
        currentChatId: s.currentChatId,
        messages: s.messages,
      }),
    },
  ),
);

export type { ChatStore };
