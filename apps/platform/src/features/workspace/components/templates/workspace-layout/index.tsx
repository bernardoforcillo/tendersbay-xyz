import { Link, Outlet, useParams } from '@tanstack/react-router';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { WorkspaceContext } from '~/features/workspace/context';
import { useWorkspace } from '~/features/workspace/hooks';
import { BTN_SECONDARY } from '~/features/workspace/ui';
import { useWorkspaceStore } from '~/store/workspace';

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

  return (
    <AccountLayout>
      <WorkspaceContext.Provider value={{ workspaceId, workspace, myPermissions, refetch }}>
        <div className="flex min-h-full flex-col">
          <Outlet />
        </div>
      </WorkspaceContext.Provider>
    </AccountLayout>
  );
}
