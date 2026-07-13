import { Banner, Button, Card, Pill, Select } from '@tendersbay/components/core';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useWorkspaceContext } from '~/features/workspace/context';
import { useMembers, useRoles } from '~/features/workspace/hooks';
import { can, Permission } from '~/features/workspace/permissions';
import { workspaceClient } from '~/lib/api/client';

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
      {actionError && <Banner tone="error">{actionError}</Banner>}
      {loading && (
        <p className="text-sm text-ink-500">{t('workspace.common.loading', 'Loading…')}</p>
      )}
      {error && <Banner tone="error">{error}</Banner>}
      <ul className="flex flex-col gap-2">
        {members?.map((m) => {
          const isOwner = m.userId === workspace.ownerId;
          return (
            <li key={m.userId}>
              <Card className="flex flex-wrap items-center justify-between gap-3 py-4">
                <div className="min-w-0">
                  <p className="truncate font-medium text-ink-900">
                    {m.user?.displayName || m.user?.email || m.userId}
                    {isOwner && (
                      <Pill tone="match" className="ml-2">
                        {t('workspace.members.owner', 'Owner')}
                      </Pill>
                    )}
                  </p>
                  <p className="truncate text-xs text-ink-500">{m.user?.email}</p>
                </div>
                <div className="flex items-center gap-2">
                  {canManage && !isOwner ? (
                    <Select
                      label={t('workspace.members.role', 'Role')}
                      value={m.roleId}
                      disabled={busy === m.userId}
                      onChange={(e) => changeRole(m.userId, e.target.value)}
                    >
                      {roles?.map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.name}
                        </option>
                      ))}
                    </Select>
                  ) : (
                    <span className="text-sm text-ink-600">{m.roleName}</span>
                  )}
                  {canManage && !isOwner && (
                    <Button
                      variant="danger"
                      isDisabled={busy === m.userId}
                      onPress={() => remove(m.userId)}
                    >
                      {t('workspace.members.remove', 'Remove')}
                    </Button>
                  )}
                </div>
              </Card>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
