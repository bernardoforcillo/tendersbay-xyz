import { Link, Outlet, useNavigate, useParams } from '@tanstack/react-router';
import { Button, tabClass } from '@tendersbay/components/core';
import { Settings } from 'lucide-react';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '~/features/account/components/organisms';
import { WorkbenchContext } from '~/features/workbench/context';
import { useWorkbench } from '~/features/workbench/hooks';
import { useRecentWorkbenchesStore } from '~/store/recent-workbenches';

export function WorkbenchLayout() {
  const { workspaceId, workbenchId } = useParams({
    from: '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId',
  });
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data, loading, error, refetch } = useWorkbench(workbenchId);

  const record = useRecentWorkbenchesStore((s) => s.record);
  const workbench = data?.workbench;
  useEffect(() => {
    if (workbench) {
      record({
        workbenchId: workbench.id,
        workspaceId: workbench.workspaceId,
        name: workbench.name,
      });
    }
  }, [workbench, record]);

  if (loading) {
    return (
      <>
        <PageHeader />
        <div className="p-6 text-sm text-ink-500 lg:p-8">
          {t('workbench.common.loading', 'Loading…')}
        </div>
      </>
    );
  }
  if (error || !data?.workbench) {
    return (
      <>
        <PageHeader />
        <div className="flex flex-col items-start gap-4 p-6 lg:p-8">
          <p className="text-sm text-ink-700">
            {error ?? t('workbench.errors.notFound', 'This workbench is unavailable.')}
          </p>
          <Button
            variant="ghost"
            onPress={() =>
              void navigate({
                to: '/workspaces/$workspaceId/workbenches',
                params: { workspaceId },
              })
            }
          >
            {t('workbench.nav.allWorkbenches', 'All workbenches')}
          </Button>
        </div>
      </>
    );
  }

  const { myPermissions, workspaceName } = data;
  if (!workbench) return null;

  return (
    <WorkbenchContext.Provider
      value={{ workbenchId, workbench, myPermissions, workspaceName, refetch }}
    >
      <PageHeader
        leading={
          <span
            aria-hidden="true"
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-brand-100 font-display text-lg font-semibold text-brand-700"
          >
            {workbench.name.charAt(0).toUpperCase()}
          </span>
        }
        title={workbench.name}
        subtitle={workbench.description || undefined}
        actions={
          <Link
            to="/workspaces/$workspaceId/workbench/$workbenchId/settings"
            params={{ workspaceId, workbenchId }}
            aria-label={t('workbench.nav.settings', 'Settings')}
            className={tabClass}
          >
            <Settings size={18} aria-hidden="true" />
          </Link>
        }
      />
      <div className="flex min-h-full flex-col gap-6 p-6 lg:p-8">
        <Outlet />
      </div>
    </WorkbenchContext.Provider>
  );
}
