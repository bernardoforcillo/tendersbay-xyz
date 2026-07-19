import { create } from 'zustand';

export type ToastTone = 'success' | 'error' | 'info';

export interface Toast {
  id: string;
  tone: ToastTone;
  message: string;
  duration?: number;
  action?: {
    label: string;
    onPress: () => void;
  };
}

interface ToastStore {
  toasts: Toast[];
  addToast: (toast: Omit<Toast, 'id'>) => void;
  dismissToast: (id: string) => void;
}

let counter = 0;

export const useToastStore = create<ToastStore>()((set) => ({
  toasts: [],

  addToast: (toast) =>
    set((s) => {
      const id = `toast-${++counter}`;
      const next = [...s.toasts, { ...toast, id }];
      return { toasts: next.slice(-5) };
    }),

  dismissToast: (id) =>
    set((s) => ({
      toasts: s.toasts.filter((t) => t.id !== id),
    })),
}));
