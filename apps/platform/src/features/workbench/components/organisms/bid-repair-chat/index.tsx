import { Card, cn } from '@tendersbay/components/core';
import { ChevronDown } from 'lucide-react';
import { useEffect, useRef } from 'react';
import { Button, Disclosure, DisclosurePanel } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import { ChatWindow } from '~/features/account/components/organisms';
import { useChatStore } from '~/store/chat';
import { useSchedaGaraStore } from '~/store/scheda-gara';

export type BidRepairChatProps = {
  workbenchId: string;
  /** Analytics surface. */
  location: AnalyticsLocation;
  className?: string;
};

/**
 * The assistant, demoted.
 *
 * On every other surface the chat is the product; here it is a repair tool,
 * collapsed by default and sitting last. That is the whole point of the scheda
 * gara: the facts, the evidence and the decision are readable without talking to
 * anything, and the conversation is what you reach for when a gap does not make
 * sense. A chat panel open by default on this page would put a blank prompt where
 * the answer already is.
 *
 * Open state lives in the scheda-gara store because its trigger — a gap row's
 * "why?" — is outside this panel. The seed text reuses `useChatStore.draft`, the
 * one-shot handoff the ⌘K palette already uses; adding a second mechanism for the
 * same job would be duplication, not separation.
 */
export function BidRepairChat({ workbenchId, location, className }: BidRepairChatProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const open = useSchedaGaraStore((s) => s.repairOpen);
  const setOpen = useSchedaGaraStore((s) => s.setRepairOpen);

  // Fire once per open, and infer where it came from: a pending draft means a gap
  // row seeded it, no draft means the user opened the panel themselves.
  const wasOpenRef = useRef(false);
  useEffect(() => {
    if (open === wasOpenRef.current) return;
    wasOpenRef.current = open;
    if (!open) return;
    capture('repair_chat_opened', {
      location,
      seeded_from: useChatStore.getState().draft ? 'gap' : 'manual',
    });
  }, [open, location, capture]);

  return (
    <Disclosure isExpanded={open} onExpandedChange={setOpen} className={className}>
      <Card>
        <Button slot="trigger" className={TRIGGER}>
          <ChevronDown
            aria-hidden="true"
            strokeWidth={1.75}
            className={cn('h-4 w-4 transition-transform duration-150', open && 'rotate-180')}
          />
          {t('bid.scheda.repairChat.title', 'Ask about this gara')}
        </Button>
        <DisclosurePanel>
          {/* Mounted only while open: ChatWindow resumes history and consumes the
              one-shot draft on mount, so mounting it collapsed would burn the
              seed before the reader ever saw the panel. */}
          {open && (
            <div className="mt-3">
              <ChatWindow location="bid_detail" workbenchId={workbenchId} />
            </div>
          )}
        </DisclosurePanel>
      </Card>
    </Disclosure>
  );
}

const TRIGGER =
  'flex min-h-10 w-full items-center gap-2 rounded-lg text-left text-sm font-semibold text-ink-700 outline-none ' +
  'transition-colors duration-150 data-[hovered]:text-ink-900 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';
