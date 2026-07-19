import { Banner, Button, Card, ConfirmDialog, Pill, Select } from '@tendersbay/components/core';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { MemberAdd } from '~/features/workbench/components/organisms/member-add';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useWorkbenchMembers, useWorkbenchRoles } from '~/features/workbench/hooks';
import { can, Permission } from '~/features/workbench/permissions';
import { workbenchClient } from '~/lib/api/client';
import { usePreferencesStore } from '~/store/preferences';

export function WorkbenchMembersPage() {
  const { t } = useTranslation();
  const { workbenchId, workbench, myPermissions } = useWorkbenchContext();
  const { data: members, loading, error, refetch } = useWorkbenchMembers(workbenchId);
  const { data: roles } = useWorkbenchRoles(workbenchId);
  const canManage = can(myPermissions, Permission.ManageMembers);
  const [busy, setBusy] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const shouldSkipRemove = usePreferencesStore((s) => s.shouldSkip('workbench-remove-member'));
  const setSkip = usePreferencesStore((s) => s.setSkipConfirmation);

  const defaultRoleId = roles?.find((r) => r.isDefault)?.id ?? roles?.[0]?.id ?? '';

  async function changeRole(userId: string, roleId: string) {
    setBusy(userId);
    setActionError(null);
    try {
      await workbenchClient.changeWorkbenchMemberRole({ workbenchId, userId, roleId });
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
      await workbenchClient.removeWorkbenchMember({ workbenchId, userId });
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
        {t('workbench.members.title', 'Members')}
      </h2>
      {actionError && <Banner tone="error">{actionError}</Banner>}
      {loading && (
        <p className="text-sm text-ink-500">{t('workbench.common.loading', 'Loading…')}</p>
      )}
      {error && <Banner tone="error">{error}</Banner>}
      {canManage && (
        <MemberAdd
          roleId={defaultRoleId}
          existingUserIds={members?.map((m) => m.userId) ?? []}
          onAdded={refetch}
        />
      )}
      <ul className="flex flex-col gap-2">
        {members?.map((m) => {
          const isOwner = m.userId === workbench.ownerId;
          return (
            <li key={m.userId}>
              <Card className="flex flex-wrap items-center justify-between gap-3 py-4">
                <div className="flex min-w-0 items-center gap-3">
                  <span
                    aria-hidden="true"
                    className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-cream-200 text-sm font-semibold text-ink-700"
                  >
                    {(m.user?.displayName || m.user?.email || '?').charAt(0).toUpperCase()}
                  </span>
                  <div className="min-w-0">
                    <p className="truncate font-medium text-ink-900">
                      {m.user?.displayName || m.user?.email || m.userId}
                      {isOwner && (
                        <Pill tone="match" className="ml-2">
                          {t('workbench.members.owner', 'Owner')}
                        </Pill>
                      )}
                    </p>
                    <p className="truncate text-xs text-ink-500">{m.user?.email}</p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {canManage && !isOwner ? (
                    <Select
                      label={t('workbench.members.role', 'Role')}
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
                    <ConfirmDialog
                      title={t('confirm.removeWorkbenchMember.title', 'Remove member?')}
                      description={t(
                        'confirm.removeWorkbenchMember.description',
                        'This person will lose access to the workbench immediately.',
                      )}
                      confirmLabel={t('workbench.members.remove', 'Remove')}
                      onConfirm={() => remove(m.userId)}
                      skipConfirmation={shouldSkipRemove}
                      onSkipChange={(skip) => setSkip('workbench-remove-member', skip)}
                      trigger={
                        <Button variant="danger" isDisabled={busy === m.userId}>
                          {t('workbench.members.remove', 'Remove')}
                        </Button>
                      }
                    />
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
