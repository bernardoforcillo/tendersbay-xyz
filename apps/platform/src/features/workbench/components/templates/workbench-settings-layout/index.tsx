import { Link, Outlet, useParams } from '@tanstack/react-router';
import { tabClass } from '@tendersbay/components/core';
import { useTranslation } from 'react-i18next';

export function WorkbenchSettingsLayout() {
  const { workspaceId, workbenchId } = useParams({
    from: '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId',
  });
  const { t } = useTranslation();

  return (
    <div className="flex flex-col gap-6">
      <nav className="flex flex-wrap gap-1 border-b border-cream-200 pb-2">
        <Link
          to="/workspaces/$workspaceId/workbench/$workbenchId/settings"
          params={{ workspaceId, workbenchId }}
          activeOptions={{ exact: true }}
          className={tabClass}
        >
          {t('workbench.nav.general', 'General')}
        </Link>
        <Link
          to="/workspaces/$workspaceId/workbench/$workbenchId/settings/members"
          params={{ workspaceId, workbenchId }}
          className={tabClass}
        >
          {t('workbench.nav.members', 'Members')}
        </Link>
        <Link
          to="/workspaces/$workspaceId/workbench/$workbenchId/settings/roles"
          params={{ workspaceId, workbenchId }}
          className={tabClass}
        >
          {t('workbench.nav.roles', 'Roles')}
        </Link>
      </nav>
      <Outlet />
    </div>
  );
}
