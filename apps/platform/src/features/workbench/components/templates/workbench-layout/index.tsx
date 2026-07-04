import { Link, Outlet, useParams } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { WorkbenchContext } from '~/features/workbench/context';
import { useWorkbench } from '~/features/workbench/hooks';
import { BTN_SECONDARY } from '~/features/workbench/ui';
import { Breadcrumb } from '~/features/workspace/components/molecules/breadcrumb';

const TAB =
  'rounded-lg px-3 py-2 text-sm font-medium text-ink-500 no-underline transition-colors hover:bg-cream-200 hover:text-ink-900 [&[aria-current=page]]:bg-cream-200 [&[aria-current=page]]:text-ink-900';

export function WorkbenchLayout() {
  const { workspaceId, workbenchId } = useParams({
    from: '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId',
  });
  const { t } = useTranslation();
  const { data, loading, error, refetch } = useWorkbench(workbenchId);

  if (loading) {
    return (
      <div className="p-6 text-sm text-ink-500 lg:p-8">
        {t('workbench.common.loading', 'Loading…')}
      </div>
    );
  }
  if (error || !data?.workbench) {
    return (
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
    );
  }

  const { workbench, myPermissions, workspaceName } = data;
  const tabs = [
    {
      to: '/workspaces/$workspaceId/workbench/$workbenchId',
      key: 'overview',
      label: t('workbench.nav.overview', 'Overview'),
      exact: true,
    },
    {
      to: '/workspaces/$workspaceId/workbench/$workbenchId/members',
      key: 'members',
      label: t('workbench.nav.members', 'Members'),
    },
    {
      to: '/workspaces/$workspaceId/workbench/$workbenchId/roles',
      key: 'roles',
      label: t('workbench.nav.roles', 'Roles'),
    },
    {
      to: '/workspaces/$workspaceId/workbench/$workbenchId/settings',
      key: 'settings',
      label: t('workbench.nav.settings', 'Settings'),
    },
  ] as const;

  return (
    <WorkbenchContext.Provider
      value={{ workbenchId, workbench, myPermissions, workspaceName, refetch }}
    >
      <div className="flex flex-col gap-6 p-6 lg:p-8">
        <header className="flex flex-col gap-4">
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
          <div>
            <h1 className="font-display text-2xl text-ink-900">{workbench.name}</h1>
            {workbench.description && (
              <p className="text-sm text-ink-500">{workbench.description}</p>
            )}
          </div>
          <nav className="flex flex-wrap gap-1 border-b border-cream-200 pb-2">
            {tabs.map((tab) => (
              <Link
                key={tab.key}
                to={tab.to}
                params={{ workspaceId, workbenchId }}
                activeOptions={'exact' in tab && tab.exact ? { exact: true } : undefined}
                className={TAB}
              >
                {tab.label}
              </Link>
            ))}
          </nav>
        </header>
        <Outlet />
      </div>
    </WorkbenchContext.Provider>
  );
}
