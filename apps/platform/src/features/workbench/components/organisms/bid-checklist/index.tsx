import { Card, Field, Select } from '@tendersbay/components/core';
import type { ChecklistItem } from '@tendersbay/proto/bid/v1/bid_pb';
import { usePostHog } from 'posthog-js/react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { bidClient } from '~/lib/api/client';

export type BidChecklistProps = {
  workbenchId: string;
  bidId: string;
  items: ChecklistItem[];
  done: number;
  total: number;
  canManage: boolean;
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Refetch the checklist and the bid (progress counters live on the bid). */
  onChanged: () => void;
  className?: string;
};

/**
 * The ESPD/DGUE checklist, lifted out of the bid detail page unchanged.
 *
 * Behaviour is deliberately identical to what it was inline: the scheda gara is a
 * re-composition, not a rewrite, and mixing "moved" with "changed" in one step
 * makes any regression impossible to attribute. Sections keep their server-given
 * order (`sectionOrder` is derived from the items as they arrive, not sorted).
 */
export function BidChecklist({
  workbenchId,
  bidId,
  items,
  done,
  total,
  canManage,
  location,
  onChanged,
  className,
}: BidChecklistProps) {
  const { t } = useTranslation();
  const posthog = usePostHog();

  const grouped: Record<string, ChecklistItem[]> = {};
  for (const item of items) {
    const list = grouped[item.sectionCode] ?? [];
    list.push(item);
    grouped[item.sectionCode] = list;
  }
  const sectionOrder = [...new Set(items.map((i) => i.sectionCode))];

  async function saveItem(item: ChecklistItem, status: string, note: string) {
    await bidClient.upsertChecklistAnswer({
      workbenchId,
      bidId,
      itemCode: item.itemCode,
      status,
      note,
    });
    if (status === 'done') {
      posthog?.capture('checklist_item_completed', {
        location,
        section_code: item.sectionCode,
        required: item.required,
      });
    }
    onChanged();
  }

  return (
    <Card className={className}>
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-ink-700">
          {t('bid.checklist.title', 'ESPD / DGUE checklist')}
        </h2>
        <span className="text-xs text-ink-500">{t('bid.checklist.progress', { done, total })}</span>
      </div>
      <div className="mt-3 flex flex-col gap-4">
        {sectionOrder.map((section) => (
          <div key={section}>
            <p className="text-xs font-semibold uppercase tracking-wide text-ink-400">
              {t(`bid.checklist.sections.${section}`, section)}
            </p>
            <div className="mt-2 flex flex-col gap-2">
              {(grouped[section] ?? []).map((item) => (
                <ChecklistRow key={item.id} item={item} canManage={canManage} onSave={saveItem} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

function ChecklistRow({
  item,
  canManage,
  onSave,
}: {
  item: ChecklistItem;
  canManage: boolean;
  onSave: (item: ChecklistItem, status: string, note: string) => void;
}) {
  const { t } = useTranslation();
  const [note, setNote] = useState(item.note);
  return (
    <div className="flex flex-col gap-2 rounded-xl border border-cream-200 p-3 sm:flex-row sm:items-end">
      <div className="flex-1">
        <p className="text-sm text-ink-900">
          {t(`bid.checklist.items.${item.itemCode}`, item.itemCode)}
        </p>
        {item.required && (
          <span className="text-[10px] font-semibold uppercase tracking-wide text-ink-400">
            {t('bid.checklist.required', 'Required')}
          </span>
        )}
      </div>
      <Select
        label={t('bid.checklist.statusLabel', 'Status')}
        className="w-32"
        value={item.status}
        disabled={!canManage}
        onChange={(e) => onSave(item, e.target.value, note)}
      >
        <option value="pending">{t('bid.checklist.status.pending', 'Pending')}</option>
        <option value="done">{t('bid.checklist.status.done', 'Done')}</option>
        <option value="na">{t('bid.checklist.status.na', 'N/A')}</option>
      </Select>
      <Field
        label={t('bid.checklist.noteLabel', 'Note')}
        placeholder={t('bid.checklist.notePlaceholder', 'Add a note…')}
        className="flex-1"
        value={note}
        onChange={setNote}
        isDisabled={!canManage}
        onBlur={() => {
          if (note !== item.note) onSave(item, item.status, note);
        }}
      />
    </div>
  );
}
