import { Link, useParams } from '@tanstack/react-router';
import { Building2, ChevronsUpDown, Plus } from 'lucide-react';
import { Button, Dialog, DialogTrigger, Popover } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useMyWorkspaces } from '~/features/workspace/hooks';
import { useWorkspaceStore } from '~/store/workspace';

const POPUP_LINK =
  'flex items-center gap-2.5 rounded-md px-2 py-1.5 text-sm no-underline text-ink-700 transition-colors hover:bg-cream-100 hover:text-ink-900';

export function WorkspaceSwitcher() {
  const { t } = useTranslation();
  const { data: workspaces } = useMyWorkspaces();
  const { workspaceId: routeWorkspaceId } = useParams({ strict: false });
  const currentWorkspaceId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const activeId = routeWorkspaceId ?? currentWorkspaceId;
  const current = workspaces?.find((w) => w.id === activeId);
  const label = current?.name ?? t('workspace.switcher.label', 'Workspaces');

  return (
    <DialogTrigger>
      <Button className="flex w-full items-center gap-2.5 rounded-xl px-3 py-2 text-left outline-none transition data-[hovered]:bg-cream-200 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600">
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-brand-100 text-brand-700">
          <Building2 size={15} aria-hidden="true" />
        </span>
        <span className="min-w-0 flex-1 truncate text-sm font-medium text-ink-800">{label}</span>
        <ChevronsUpDown size={14} className="shrink-0 text-ink-400" aria-hidden="true" />
      </Button>

      <Popover
        placement="bottom start"
        offset={6}
        className="z-50 w-60 rounded-xl border border-cream-200 bg-white shadow-soft outline-none"
      >
        <Dialog aria-label={t('workspace.switcher.label', 'Workspaces')} className="outline-none">
          {workspaces && workspaces.length > 0 && (
            <div className="max-h-64 overflow-y-auto p-1.5">
              {workspaces.map((w) => (
                <Link
                  key={w.id}
                  to="/workspaces/$workspaceId"
                  params={{ workspaceId: w.id }}
                  className={POPUP_LINK}
                >
                  <Building2 size={14} aria-hidden="true" className="shrink-0 text-ink-400" />
                  <span className="truncate">{w.name}</span>
                </Link>
              ))}
            </div>
          )}
          <div className="border-t border-cream-200 p-1.5">
            <Link to="/workspaces" className={POPUP_LINK}>
              <Plus size={14} aria-hidden="true" className="shrink-0 text-ink-400" />
              {t('workspace.switcher.manage', 'All workspaces')}
            </Link>
          </div>
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
