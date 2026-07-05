import { Link, Outlet, useParams } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

const TAB =
  'rounded-lg px-3 py-2 text-sm font-medium text-ink-500 no-underline transition-colors hover:bg-cream-200 hover:text-ink-900 [&[aria-current=page]]:bg-cream-200 [&[aria-current=page]]:text-ink-900';

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
          className={TAB}
        >
          {t('workbench.nav.general', 'General')}
        </Link>
        <Link
          to="/workspaces/$workspaceId/workbench/$workbenchId/settings/members"
          params={{ workspaceId, workbenchId }}
          className={TAB}
        >
          {t('workbench.nav.members', 'Members')}
        </Link>
        <Link
          to="/workspaces/$workspaceId/workbench/$workbenchId/settings/roles"
          params={{ workspaceId, workbenchId }}
          className={TAB}
        >
          {t('workbench.nav.roles', 'Roles')}
        </Link>
      </nav>
      <Outlet />
    </div>
  );
}
