import { useCallback } from 'react';
import { type Toast, type ToastTone, useToastStore } from '~/store/toast';

export function useToast() {
  const addToast = useToastStore((s) => s.addToast);

  const toast = useCallback(
    (
      tone: ToastTone,
      message: string,
      options?: { duration?: number; action?: Toast['action'] },
    ) => {
      addToast({ tone, message, ...options });
    },
    [addToast],
  );

  return {
    success: useCallback(
      (msg: string, opts?: { duration?: number; action?: Toast['action'] }) =>
        toast('success', msg, opts),
      [toast],
    ),
    error: useCallback(
      (msg: string, opts?: { duration?: number; action?: Toast['action'] }) =>
        toast('error', msg, opts),
      [toast],
    ),
    info: useCallback(
      (msg: string, opts?: { duration?: number; action?: Toast['action'] }) =>
        toast('info', msg, opts),
      [toast],
    ),
  };
}
