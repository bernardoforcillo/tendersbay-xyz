import { useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { MemberAdd } from '~/features/workbench/components/organisms/member-add';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useWorkbenchMembers, useWorkbenchRoles } from '~/features/workbench/hooks';
import { can, Permission } from '~/features/workbench/permissions';
import { BTN_DANGER, CARD, ERROR_BOX } from '~/features/workbench/ui';
import { workbenchClient } from '~/lib/api/client';

const SELECT =
  'rounded-lg border border-cream-300 bg-cream-50 px-2.5 py-1.5 text-sm text-ink-800 outline-none transition focus:border-brand-400 focus:ring-2 focus:ring-brand-100 disabled:opacity-50';

export function WorkbenchMembersPage() {
  const { t } = useTranslation();
  const { workbenchId, workbench, myPermissions } = useWorkbenchContext();
  const { data: members, loading, error, refetch } = useWorkbenchMembers(workbenchId);
  const { data: roles } = useWorkbenchRoles(workbenchId);
  const canManage = can(myPermissions, Permission.ManageMembers);
  const [busy, setBusy] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

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
      {actionError && (
        <p role="alert" className={ERROR_BOX}>
          {actionError}
        </p>
      )}
      {loading && (
        <p className="text-sm text-ink-500">{t('workbench.common.loading', 'Loading…')}</p>
      )}
      {error && (
        <p role="alert" className={ERROR_BOX}>
          {error}
        </p>
      )}
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
            <li
              key={m.userId}
              className={`${CARD} flex flex-wrap items-center justify-between gap-3 py-4`}
            >
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
                      <span className="ml-2 rounded-full bg-brand-100 px-2 py-0.5 text-xs font-medium text-brand-700">
                        {t('workbench.members.owner', 'Owner')}
                      </span>
                    )}
                  </p>
                  <p className="truncate text-xs text-ink-500">{m.user?.email}</p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {canManage && !isOwner ? (
                  <select
                    aria-label={t('workbench.members.role', 'Role')}
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
                    {t('workbench.members.remove', 'Remove')}
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
