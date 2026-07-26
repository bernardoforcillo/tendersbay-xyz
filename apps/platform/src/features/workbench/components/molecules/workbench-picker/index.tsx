import { Banner, Card, EmptyState, Pill } from '@tendersbay/components/core';
import { Lock, Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useWorkbenches } from '~/features/workbench/hooks';
import { useRecentWorkbenchesStore } from '~/store/recent-workbenches';

export function WorkbenchPicker({
  workspaceId,
  onSelect,
  busyId,
}: {
  workspaceId: string;
  onSelect: (workbenchId: string) => void;
  busyId?: string | null;
}) {
  const { t } = useTranslation();
  const { data: workbenches, loading, error } = useWorkbenches(workspaceId);
  const recentIds = useRecentWorkbenchesStore((s) => s.items)
    .filter((i) => i.workspaceId === workspaceId)
    .map((i) => i.workbenchId);

  if (loading)
    return <p className="text-sm text-ink-500">{t('workbench.common.loading', 'Loading…')}</p>;
  if (error) return <Banner tone="error">{error}</Banner>;
  if (!workbenches || workbenches.length === 0)
    return <EmptyState title={t('bid.picker.empty', 'No workbenches yet — create one first.')} />;

  const ordered = [
    ...recentIds
      .map((id) => workbenches.find((w) => w.id === id))
      .filter((w): w is (typeof workbenches)[number] => Boolean(w)),
    ...workbenches.filter((w) => !recentIds.includes(w.id)),
  ];

  return (
    <ul className="flex flex-col gap-2">
      {ordered.map((wb) => (
        <li key={wb.id}>
          <Card padding="none">
            <button
              type="button"
              disabled={busyId === wb.id}
              onClick={() => onSelect(wb.id)}
              className="flex w-full items-center gap-4 rounded-2xl p-4 text-left outline-none transition-colors duration-150 hover:bg-cream-50 focus-visible:ring-2 focus-visible:ring-brand-600 disabled:opacity-50"
            >
              <span
                aria-hidden="true"
                className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-100 font-display text-base font-semibold text-brand-700"
              >
                {wb.name.charAt(0).toUpperCase()}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate font-medium text-ink-900">{wb.name}</span>
                {wb.description && (
                  <span className="block truncate text-xs text-ink-500">{wb.description}</span>
                )}
              </span>
              <Pill tone="neutral" className="shrink-0 gap-1.5">
                {wb.visibility === 'shared' ? (
                  <Users size={14} aria-hidden="true" />
                ) : (
                  <Lock size={14} aria-hidden="true" />
                )}
                {t(`workbench.visibility.${wb.visibility}`, wb.visibility)}
              </Pill>
            </button>
          </Card>
        </li>
      ))}
    </ul>
  );
}
