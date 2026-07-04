import { Link, Outlet } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { useWorkspaceContext } from '~/features/workspace/context';
import { can, Permission } from '~/features/workspace/permissions';
import { settingsNavKeys } from './settings-nav';

const TAB =
  'rounded-lg px-3 py-2 text-sm font-medium text-ink-500 no-underline transition-colors hover:bg-cream-200 hover:text-ink-900 [&[aria-current=page]]:bg-cream-200 [&[aria-current=page]]:text-ink-900';

export function WorkspaceSettingsLayout() {
  const { t } = useTranslation();
  const { workspaceId, workspace, myPermissions } = useWorkspaceContext();
  const canInvite =
    can(myPermissions, Permission.CreateInvite) || can(myPermissions, Permission.ManageInvites);
  const keys = settingsNavKeys(canInvite);

  return (
    <div className="flex flex-col gap-6 p-6 lg:p-8">
      <header className="flex flex-col gap-4">
        <div>
          <h1 className="font-display text-2xl text-ink-900">{workspace.name}</h1>
          <p className="text-sm text-ink-500">/{workspace.slug}</p>
        </div>
        <nav className="flex flex-wrap gap-1 border-b border-cream-200 pb-2">
          {keys.includes('general') && (
            <Link
              to="/workspaces/$workspaceId/settings"
              params={{ workspaceId }}
              activeOptions={{ exact: true }}
              className={TAB}
            >
              {t('workspace.nav.general', 'General')}
            </Link>
          )}
          {keys.includes('members') && (
            <Link
              to="/workspaces/$workspaceId/settings/members"
              params={{ workspaceId }}
              className={TAB}
            >
              {t('workspace.nav.members', 'Members')}
            </Link>
          )}
          {keys.includes('roles') && (
            <Link
              to="/workspaces/$workspaceId/settings/roles"
              params={{ workspaceId }}
              className={TAB}
            >
              {t('workspace.nav.roles', 'Roles')}
            </Link>
          )}
          {keys.includes('invites') && (
            <Link
              to="/workspaces/$workspaceId/settings/invites"
              params={{ workspaceId }}
              className={TAB}
            >
              {t('workspace.nav.invites', 'Invites')}
            </Link>
          )}
        </nav>
      </header>
      <Outlet />
    </div>
  );
}
