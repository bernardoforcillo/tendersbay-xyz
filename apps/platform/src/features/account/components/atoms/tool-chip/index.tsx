import { Check, Loader2 } from 'lucide-react';
import type { FC } from 'react';
import { useTranslation } from 'react-i18next';

export type ToolChipProps = {
  name: string;
  status: 'running' | 'done';
};

// Known tools get a localized running/done label; an unknown tool falls back
// to its raw name so a newly-added tool still shows a breadcrumb.
const LABELS: Record<string, { running: [string, string]; done: [string, string] }> = {
  search_tenders: {
    running: ['account.explore.tool.searchTenders.running', 'Cerco bandi…'],
    done: ['account.explore.tool.searchTenders.done', 'Bandi trovati'],
  },
  create_workbench: {
    running: ['account.explore.tool.createWorkbench.running', 'Creo il workbench…'],
    done: ['account.explore.tool.createWorkbench.done', 'Workbench creato'],
  },
};

export const ToolChip: FC<ToolChipProps> = ({ name, status }) => {
  const { t } = useTranslation();
  const entry = LABELS[name];
  const label = entry ? t(entry[status][0], { defaultValue: entry[status][1] }) : name;

  return (
    <span
      className={
        'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium ' +
        (status === 'done'
          ? 'border-cream-300 bg-cream-100 text-ink-500'
          : 'border-brand-200 bg-brand-50 text-brand-700')
      }
    >
      {status === 'done' ? (
        <Check size={12} className="text-brand-600" aria-hidden />
      ) : (
        <Loader2 size={12} className="animate-spin" aria-hidden />
      )}
      {label}
    </span>
  );
};
