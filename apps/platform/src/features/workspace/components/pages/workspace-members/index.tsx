import { useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useWorkspaceContext } from '~/features/workspace/context';
import { useMembers, useRoles } from '~/features/workspace/hooks';
import { can, Permission } from '~/features/workspace/permissions';
import { BTN_DANGER, CARD, ERROR_BOX } from '~/features/workspace/ui';
import { workspaceClient } from '~/lib/api/client';

const SELECT =
  'rounded-lg border border-cream-300 bg-cream-50 px-2.5 py-1.5 text-sm text-ink-800 outline-none transition focus:border-brand-400 focus:ring-2 focus:ring-brand-100 disabled:opacity-50';

export function WorkspaceMembersPage() {
  const { t } = useTranslation();
  const { workspaceId, workspace, myPermissions } = useWorkspaceContext();
  const { data: members, loading, error, refetch } = useMembers(workspaceId);
  const { data: roles } = useRoles(workspaceId);
  const canManage = can(myPermissions, Permission.ManageMembers);
  const [busy, setBusy] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  async function changeRole(userId: string, roleId: string) {
    setBusy(userId);
    setActionError(null);
    try {
      await workspaceClient.changeMemberRole({ workspaceId, userId, roleId });
      refetch();
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : 'Failed to change role');
    } finally {
      setBusy(null);
    }
  }

  async function remove(userId: string) {
    setBusy(userId);
    setActionError(null);
    try {
      await workspaceClient.removeMember({ workspaceId, userId });
      refetch();
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : 'Failed to remove member');
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h2 className="font-display text-lg text-ink-900">
        {t('workspace.members.title', 'Members')}
      </h2>
      {actionError && (
        <p role="alert" className={ERROR_BOX}>
          {actionError}
        </p>
      )}
      {loading && (
        <p className="text-sm text-ink-500">{t('workspace.common.loading', 'Loading…')}</p>
      )}
      {error && (
        <p role="alert" className={ERROR_BOX}>
          {error}
        </p>
      )}
      <ul className="flex flex-col gap-2">
        {members?.map((m) => {
          const isOwner = m.userId === workspace.ownerId;
          return (
            <li
              key={m.userId}
              className={`${CARD} flex flex-wrap items-center justify-between gap-3 py-4`}
            >
              <div className="min-w-0">
                <p className="truncate font-medium text-ink-900">
                  {m.user?.displayName || m.user?.email || m.userId}
                  {isOwner && (
                    <span className="ml-2 rounded-full bg-brand-100 px-2 py-0.5 text-xs font-medium text-brand-700">
                      {t('workspace.members.owner', 'Owner')}
                    </span>
                  )}
                </p>
                <p className="truncate text-xs text-ink-500">{m.user?.email}</p>
              </div>
              <div className="flex items-center gap-2">
                {canManage && !isOwner ? (
                  <select
                    aria-label={t('workspace.members.role', 'Role')}
                    className={SELECT}
                    value={m.roleId}
                    disabled={busy === m.userId}
                    onChange={(e) => changeRole(m.userId, e.target.value)}
                  >
                    {roles?.map((r) => (
                      <option key={r.id} value={r.id}>
                        {r.name}
                      </option>
                    ))}
                  </select>
                ) : (
                  <span className="text-sm text-ink-600">{m.roleName}</span>
                )}
                {canManage && !isOwner && (
                  <Button
                    className={BTN_DANGER}
                    isDisabled={busy === m.userId}
                    onPress={() => remove(m.userId)}
                  >
                    {t('workspace.members.remove', 'Remove')}
                  </Button>
                )}
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
