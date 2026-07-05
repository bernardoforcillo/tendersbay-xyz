import type { Role } from '@tendersbay/proto/workbench/v1/workbench_pb';
import { ShieldCheck } from 'lucide-react';
import { useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { RoleEditor } from '~/features/workbench/components/organisms/role-editor';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useWorkbenchRoles } from '~/features/workbench/hooks';
import {
  can,
  hasBit,
  PERMISSION_BITS,
  PERMISSION_KEYS,
  Permission,
} from '~/features/workbench/permissions';
import { BTN_DANGER, BTN_PRIMARY, BTN_SECONDARY, CARD, ERROR_BOX } from '~/features/workbench/ui';
import { workbenchClient } from '~/lib/api/client';

function permissionCount(perms: bigint): number {
  return PERMISSION_KEYS.filter((k) => hasBit(perms, PERMISSION_BITS[k])).length;
}

export function WorkbenchRolesPage() {
  const { t } = useTranslation();
  const { workbenchId, myPermissions } = useWorkbenchContext();
  const { data: roles, loading, error, refetch } = useWorkbenchRoles(workbenchId);
  const canManage = can(myPermissions, Permission.ManageRoles);
  const [editing, setEditing] = useState<Role | 'new' | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  async function submit(name: string, permissions: bigint) {
    setSubmitting(true);
    setFormError(null);
    try {
      if (editing === 'new') {
        await workbenchClient.createWorkbenchRole({ workbenchId, name, permissions });
      } else if (editing) {
        await workbenchClient.updateWorkbenchRole({
          workbenchId,
          roleId: editing.id,
          name,
          permissions,
        });
      }
      setEditing(null);
      refetch();
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : 'Failed to save role');
    } finally {
      setSubmitting(false);
    }
  }

  async function remove(roleId: string) {
    setActionError(null);
    try {
      await workbenchClient.deleteWorkbenchRole({ workbenchId, roleId });
      refetch();
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : 'Failed to delete role');
    }
  }

  if (editing) {
    return (
      <div className="flex flex-col gap-4">
        <h2 className="font-display text-lg text-ink-900">
          {editing === 'new'
            ? t('workbench.roles.createTitle', 'New role')
            : t('workbench.roles.editTitle', 'Edit role')}
        </h2>
        <div className={CARD}>
          <RoleEditor
            initialName={editing === 'new' ? '' : editing.name}
            initialPermissions={editing === 'new' ? 0n : editing.permissions}
            submitting={submitting}
            error={formError}
            canGrant={(bit) => can(myPermissions, bit)}
            onSubmit={submit}
            onCancel={() => setEditing(null)}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="font-display text-lg text-ink-900">{t('workbench.roles.title', 'Roles')}</h2>
        {canManage && (
          <Button className={BTN_PRIMARY} onPress={() => setEditing('new')}>
            {t('workbench.roles.new', 'New role')}
          </Button>
        )}
      </div>
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
      <ul className="flex flex-col gap-2">
        {roles?.map((r) => (
          <li
            key={r.id}
            className={`${CARD} flex flex-wrap items-center justify-between gap-3 py-4`}
          >
            <div className="flex min-w-0 items-center gap-3">
              <span
                aria-hidden="true"
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-cream-200 text-ink-500"
              >
                <ShieldCheck size={16} />
              </span>
              <div className="min-w-0">
                <p className="font-medium text-ink-900">
                  {r.name}
                  {r.isDefault && (
                    <span className="ml-2 rounded-full bg-brand-100 px-2 py-0.5 text-xs font-medium text-brand-700">
                      {t('workbench.roles.default', 'Default')}
                    </span>
                  )}
                </p>
                <p className="text-xs text-ink-500">
                  {t('workbench.roles.permCount', '{{count}} permissions', {
                    count: permissionCount(r.permissions),
                  })}
                </p>
              </div>
            </div>
            {canManage && (
              <div className="flex gap-2">
                <Button className={BTN_SECONDARY} onPress={() => setEditing(r)}>
                  {t('workbench.common.edit', 'Edit')}
                </Button>
                {!r.isDefault && (
                  <Button className={BTN_DANGER} onPress={() => remove(r.id)}>
                    {t('workbench.common.delete', 'Delete')}
                  </Button>
                )}
              </div>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
