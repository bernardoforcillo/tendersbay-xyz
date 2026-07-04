import { Link, Outlet, useParams } from '@tanstack/react-router';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { WorkspaceContext } from '~/features/workspace/context';
import { useWorkspace } from '~/features/workspace/hooks';
import { can, Permission } from '~/features/workspace/permissions';
import { BTN_SECONDARY } from '~/features/workspace/ui';
import { useWorkspaceStore } from '~/store/workspace';

const TAB =
  'rounded-lg px-3 py-2 text-sm font-medium text-ink-500 no-underline transition-colors hover:bg-cream-200 hover:text-ink-900 [&[aria-current=page]]:bg-cream-200 [&[aria-current=page]]:text-ink-900';

export function WorkspaceLayout() {
  const { workspaceId } = useParams({ from: '/_authenticated/workspaces/$workspaceId' });
  const { t } = useTranslation();
  const { data, loading, error, refetch } = useWorkspace(workspaceId);
  const setCurrentWorkspace = useWorkspaceStore((s) => s.setCurrentWorkspace);

  useEffect(() => {
    if (data?.workspace) setCurrentWorkspace(workspaceId);
  }, [data?.workspace, workspaceId, setCurrentWorkspace]);

  if (loading) {
    return (
      <AccountLayout>
        <div className="p-8 text-sm text-ink-500">{t('workspace.common.loading', 'Loading…')}</div>
      </AccountLayout>
    );
  }

  if (error || !data?.workspace) {
    return (
      <AccountLayout>
        <div className="flex flex-col items-start gap-4 p-8">
          <p className="text-sm text-ink-700">
            {error ?? t('workspace.errors.notFound', 'This workspace is unavailable.')}
          </p>
          <Link to="/workspaces" className={BTN_SECONDARY}>
            {t('workspace.nav.allWorkspaces', 'All workspaces')}
          </Link>
        </div>
      </AccountLayout>
    );
  }

  const { workspace, myPermissions } = data;
  const canInvite =
    can(myPermissions, Permission.CreateInvite) || can(myPermissions, Permission.ManageInvites);

  const tabs = [
    {
      to: '/workspaces/$workspaceId',
      key: 'overview',
      label: t('workspace.nav.overview', 'Overview'),
      exact: true,
      show: true,
    },
    {
      to: '/workspaces/$workspaceId/members',
      key: 'members',
      label: t('workspace.nav.members', 'Members'),
      show: true,
    },
    {
      to: '/workspaces/$workspaceId/roles',
      key: 'roles',
      label: t('workspace.nav.roles', 'Roles'),
      show: true,
    },
    {
      to: '/workspaces/$workspaceId/invites',
      key: 'invites',
      label: t('workspace.nav.invites', 'Invites'),
      show: canInvite,
    },
    {
      to: '/workspaces/$workspaceId/settings',
      key: 'settings',
      label: t('workspace.nav.settings', 'Settings'),
      show: true,
    },
  ] as const;

  return (
    <AccountLayout>
      <WorkspaceContext.Provider value={{ workspaceId, workspace, myPermissions, refetch }}>
        <div className="flex flex-col gap-6 p-6 lg:p-8">
          <header className="flex flex-col gap-4">
            <div>
              <h1 className="font-display text-2xl text-ink-900">{workspace.name}</h1>
              <p className="text-sm text-ink-500">/{workspace.slug}</p>
            </div>
            <nav className="flex flex-wrap gap-1 border-b border-cream-200 pb-2">
              {tabs
                .filter((tab) => tab.show)
                .map((tab) => (
                  <Link
                    key={tab.key}
                    to={tab.to}
                    params={{ workspaceId }}
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
      </WorkspaceContext.Provider>
    </AccountLayout>
  );
}
