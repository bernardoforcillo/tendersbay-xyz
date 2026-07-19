import { cn } from '@tendersbay/components/core';
import { useEffect, useRef } from 'react';
import { type Toast as ToastType, useToastStore } from '~/store/toast';

const TONES: Record<ToastType['tone'], string> = {
  success: 'border-brand-200 bg-brand-50 text-brand-800',
  error: 'border-red-200 bg-red-50 text-red-700',
  info: 'border-cream-300 bg-white text-ink-800',
};

const DEFAULT_DURATION = 4000;

export function ToastItem({ toast }: { toast: ToastType }) {
  const dismiss = useToastStore((s) => s.dismissToast);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    timerRef.current = setTimeout(() => {
      dismiss(toast.id);
    }, toast.duration ?? DEFAULT_DURATION);

    return () => {
      if (timerRef.current != null) clearTimeout(timerRef.current);
    };
  }, [toast.id, toast.duration, dismiss]);

  return (
    <div
      role={toast.tone === 'error' ? 'alert' : 'status'}
      className={cn(
        'flex items-center gap-3 rounded-xl border px-4 py-3 text-sm shadow-soft',
        TONES[toast.tone],
      )}
    >
      <span className="flex-1">{toast.message}</span>
      {toast.action && (
        <button
          type="button"
          onClick={() => {
            toast.action?.onPress();
            dismiss(toast.id);
          }}
          className="font-semibold underline underline-offset-2 hover:opacity-80"
        >
          {toast.action.label}
        </button>
      )}
      <button
        type="button"
        onClick={() => dismiss(toast.id)}
        aria-label="Dismiss"
        className="ml-2 shrink-0 text-ink-400 hover:text-ink-700"
      >
        <svg
          aria-hidden="true"
          className="h-4 w-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        >
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      </button>
    </div>
  );
}
