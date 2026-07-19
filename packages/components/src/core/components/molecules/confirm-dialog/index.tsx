import { type ReactNode, useCallback, useRef, useState } from 'react';
import { Button, Checkbox, Dialog, Modal, ModalOverlay } from 'react-aria-components';
import { cn } from '../../../cn';

type ConfirmTone = 'danger' | 'neutral';

export type ConfirmDialogProps = {
  title: string;
  description?: string;
  confirmLabel: string;
  cancelLabel?: string;
  tone?: ConfirmTone;
  onConfirm: () => void;
  /** When provided the component renders the trigger and manages open state internally. */
  trigger?: ReactNode;
  /** Called when "Don't ask again" is toggled. */
  onSkipChange?: (skip: boolean) => void;
  /** When true the dialog is bypassed — onConfirm fires immediately on trigger press. */
  skipConfirmation?: boolean;
  /** Controlled open state — use when trigger is NOT provided. */
  isOpen?: boolean;
  /** Controlled open callback — use when trigger is NOT provided. */
  onOpenChange?: (open: boolean) => void;
};

const OVERLAY = 'fixed inset-0 z-50 flex items-center justify-center bg-ink-950/25 p-4';

const MODAL =
  'w-full max-w-md overflow-hidden rounded-2xl border border-cream-200 bg-white p-6 shadow-soft-lg outline-none';

const CONFIRM_VARIANT: Record<ConfirmTone, string> = {
  danger: 'bg-red-600 text-white data-[hovered]:bg-red-700 data-[pressed]:bg-red-800',
  neutral:
    'border border-cream-300 bg-white text-ink-800 data-[hovered]:border-cream-400 data-[pressed]:bg-cream-100',
};

export function ConfirmDialog({
  title,
  description,
  confirmLabel,
  cancelLabel = 'Cancel',
  tone = 'danger',
  onConfirm,
  trigger,
  onSkipChange,
  skipConfirmation = false,
  isOpen: controlledOpen,
  onOpenChange: controlledOnOpenChange,
}: ConfirmDialogProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : internalOpen;
  const setOpen = isControlled
    ? (next: boolean) => controlledOnOpenChange?.(next)
    : setInternalOpen;

  const onConfirmRef = useRef(onConfirm);
  onConfirmRef.current = onConfirm;

  const handleConfirm = useCallback(() => {
    setOpen(false);
    onConfirmRef.current();
  }, [setOpen]);

  const handleTrigger = useCallback(() => {
    if (skipConfirmation) {
      onConfirmRef.current();
    } else {
      setOpen(true);
    }
  }, [skipConfirmation, setOpen]);

  return (
    <>
      {trigger && (
        <button type="button" onClick={handleTrigger} className="inline-flex cursor-pointer">
          {trigger}
        </button>
      )}
      <ModalOverlay isOpen={open} onOpenChange={setOpen} isDismissable className={OVERLAY}>
        <Modal className={MODAL}>
          <Dialog aria-label={title} className="outline-none">
            <div className="flex flex-col gap-4">
              <div>
                <h3 className="font-display text-lg text-ink-900">{title}</h3>
                {description && <p className="mt-1 text-sm text-ink-500">{description}</p>}
              </div>
              {onSkipChange && (
                <Checkbox
                  onChange={(checked) => onSkipChange(checked)}
                  className="flex items-center gap-2 text-sm text-ink-600"
                >
                  <div className="flex h-4 w-4 items-center justify-center rounded border border-cream-400 bg-white data-[selected]:border-brand-600 data-[selected]:bg-brand-600">
                    <svg
                      aria-hidden="true"
                      className="h-3 w-3 text-white"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="3"
                    >
                      <path d="M5 13l4 4L19 7" />
                    </svg>
                  </div>
                  <span>Don't ask again</span>
                </Checkbox>
              )}
              <div className="flex justify-end gap-3">
                <Button
                  onPress={() => setOpen(false)}
                  className="inline-flex h-10 items-center justify-center rounded-xl border border-cream-300 bg-white px-4 text-sm font-semibold text-ink-800 transition-colors duration-150 data-[hovered]:border-cream-400 data-[pressed]:bg-cream-100 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-100"
                >
                  {cancelLabel}
                </Button>
                <Button
                  onPress={handleConfirm}
                  className={cn(
                    'inline-flex h-10 items-center justify-center rounded-xl px-4 text-sm font-semibold transition-colors duration-150 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-100',
                    CONFIRM_VARIANT[tone],
                  )}
                >
                  {confirmLabel}
                </Button>
              </div>
            </div>
          </Dialog>
        </Modal>
      </ModalOverlay>
    </>
  );
}
