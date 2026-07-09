import { Link, Outlet, useParams } from '@tanstack/react-router';
import { Settings } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '~/features/account/components/organisms';
import { WorkbenchContext } from '~/features/workbench/context';
import { useWorkbench } from '~/features/workbench/hooks';
import { BTN_SECONDARY } from '~/features/workbench/ui';
import { Breadcrumb } from '~/features/workspace/components/molecules/breadcrumb';

const GEAR =
  'shrink-0 rounded-lg p-2 text-ink-400 no-underline transition-colors hover:bg-cream-200 hover:text-ink-900 [&[aria-current=page]]:bg-cream-200 [&[aria-current=page]]:text-ink-900';

export function WorkbenchLayout() {
  const { workspaceId, workbenchId } = useParams({
    from: '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId',
  });
  const { t } = useTranslation();
  const { data, loading, error, refetch } = useWorkbench(workbenchId);

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
          <Link
            to="/workspaces/$workspaceId/workbenches"
            params={{ workspaceId }}
            className={BTN_SECONDARY}
          >
            {t('workbench.nav.allWorkbenches', 'All workbenches')}
          </Link>
        </div>
      </>
    );
  }

  const { workbench, myPermissions, workspaceName } = data;

  return (
    <WorkbenchContext.Provider
      value={{ workbenchId, workbench, myPermissions, workspaceName, refetch }}
    >
      <PageHeader
        breadcrumb={
          <Breadcrumb
            items={[
              { label: t('workbench.breadcrumb.workspaces', 'Workspaces'), to: '/workspaces' },
              { label: workspaceName, to: '/workspaces/$workspaceId', params: { workspaceId } },
              {
                label: t('workbench.breadcrumb.workbenches', 'Workbenches'),
                to: '/workspaces/$workspaceId/workbenches',
                params: { workspaceId },
              },
              { label: workbench.name },
            ]}
          />
        }
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
            className={GEAR}
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
