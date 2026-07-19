import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'choice_prompt' | 'choice_response' | 'tender_results';
  content: string;
  createdAt: string;
  choices?: { key: string; label: string; description: string }[];
  tenders?: TenderResult[];
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
  pendingChoice: {
    id: string;
    question: string;
    options: { key: string; label: string; description: string }[];
    allowCustom: boolean;
  } | null;
  setCurrentChat: (id: string | null) => void;
  addMessage: (msg: ChatMessage) => void;
  setMessages: (messages: ChatMessage[]) => void;
  setStreaming: (v: boolean) => void;
  appendStreamToken: (token: string) => void;
  setStreamingContent: (content: string) => void;
  setCredits: (credits: ChatStore['credits']) => void;
  setPendingChoice: (choice: ChatStore['pendingChoice']) => void;
  reset: () => void;
  /** One-shot message handed off from the ⌘K palette; consumed by ChatWindow on mount. */
  draft: string | null;
  setDraft: (draft: string | null) => void;
}

export const useChatStore = create<ChatStore>()(
  persist(
    (set) => ({
      currentChatId: null,
      messages: [],
      streaming: false,
      streamingContent: '',
      credits: null,
      pendingChoice: null,
      setCurrentChat: (id) => set({ currentChatId: id }),
      addMessage: (msg) =>
        set((s) =>
          s.messages.some((m) => m.id === msg.id) ? s : { messages: [...s.messages, msg] },
        ),
      setMessages: (messages) => set({ messages }),
      setStreaming: (v) => set({ streaming: v }),
      appendStreamToken: (token) => set((s) => ({ streamingContent: s.streamingContent + token })),
      setStreamingContent: (content) => set({ streamingContent: content }),
      setCredits: (credits) => set({ credits }),
      setPendingChoice: (pendingChoice) => set({ pendingChoice }),
      reset: () =>
        set({
          messages: [],
          streaming: false,
          streamingContent: '',
          currentChatId: null,
          pendingChoice: null,
        }),
      draft: null,
      setDraft: (draft) => set({ draft }),
    }),
    {
      name: 'chat',
      // A `tender_results` message's `tenders[].value` is a protobuf int64,
      // typed `bigint` — JSON has no native bigint literal, so the default
      // (de)serializer throws "Do not know how to serialize a BigInt" the
      // moment one lands in the persisted `messages` array. Round-trip it
      // through a tagged object instead of losing precision via Number().
      storage: createJSONStorage(() => sessionStorage, {
        replacer: (_key, value) =>
          typeof value === 'bigint' ? { __type: 'bigint', value: value.toString() } : value,
        reviver: (_key, value) =>
          value && typeof value === 'object' && (value as { __type?: string }).__type === 'bigint'
            ? BigInt((value as { value: string }).value)
            : value,
      }),
      partialize: (s) => ({
        currentChatId: s.currentChatId,
        messages: s.messages,
      }),
    },
  ),
);

export type { ChatStore };
