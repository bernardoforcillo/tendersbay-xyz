import { Link, Outlet } from '@tanstack/react-router';
import { tabClass } from '@tendersbay/components/core';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '~/features/account/components/organisms';
import { useWorkspaceContext } from '~/features/workspace/context';
import { can, Permission } from '~/features/workspace/permissions';
import { settingsNavKeys } from './settings-nav';

export function WorkspaceSettingsLayout() {
  const { t } = useTranslation();
  const { workspaceId, workspace, myPermissions } = useWorkspaceContext();
  const canInvite =
    can(myPermissions, Permission.CreateInvite) || can(myPermissions, Permission.ManageInvites);
  const keys = settingsNavKeys(canInvite);

  return (
    <>
      <PageHeader title={workspace.name} subtitle={`@${workspace.slug}`}>
        <nav className="flex flex-wrap gap-1">
          {keys.includes('general') && (
            <Link
              to="/workspaces/$workspaceId/settings"
              params={{ workspaceId }}
              activeOptions={{ exact: true }}
              className={tabClass}
            >
              {t('workspace.nav.general', 'General')}
            </Link>
          )}
          {keys.includes('profile') && (
            <Link
              to="/workspaces/$workspaceId/settings/profile"
              params={{ workspaceId }}
              className={tabClass}
            >
              {t('workspace.nav.profile', 'Client profile')}
            </Link>
          )}
          {keys.includes('members') && (
            <Link
              to="/workspaces/$workspaceId/settings/members"
              params={{ workspaceId }}
              className={tabClass}
            >
              {t('workspace.nav.members', 'Members')}
            </Link>
          )}
          {keys.includes('roles') && (
            <Link
              to="/workspaces/$workspaceId/settings/roles"
              params={{ workspaceId }}
              className={tabClass}
            >
              {t('workspace.nav.roles', 'Roles')}
            </Link>
          )}
          {keys.includes('invites') && (
            <Link
              to="/workspaces/$workspaceId/settings/invites"
              params={{ workspaceId }}
              className={tabClass}
            >
              {t('workspace.nav.invites', 'Invites')}
            </Link>
          )}
        </nav>
      </PageHeader>
      <div className="flex flex-col gap-6 p-6 lg:p-8">
        <Outlet />
      </div>
    </>
  );
}
