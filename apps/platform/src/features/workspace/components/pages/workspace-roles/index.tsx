import { Banner, Button, Card, Pill } from '@tendersbay/components/core';
import type { Role } from '@tendersbay/proto/workspace/v1/workspace_pb';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { RoleEditor } from '~/features/workspace/components/organisms/role-editor';
import { useWorkspaceContext } from '~/features/workspace/context';
import { useRoles } from '~/features/workspace/hooks';
import {
  can,
  hasBit,
  PERMISSION_BITS,
  PERMISSION_KEYS,
  Permission,
} from '~/features/workspace/permissions';
import { workspaceClient } from '~/lib/api/client';

function permissionCount(perms: bigint): number {
  return PERMISSION_KEYS.filter((k) => hasBit(perms, PERMISSION_BITS[k])).length;
}

export function WorkspaceRolesPage() {
  const { t } = useTranslation();
  const { workspaceId, myPermissions } = useWorkspaceContext();
  const { data: roles, loading, error, refetch } = useRoles(workspaceId);
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
        await workspaceClient.createRole({ workspaceId, name, permissions });
      } else if (editing) {
        await workspaceClient.updateRole({ workspaceId, roleId: editing.id, name, permissions });
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
      await workspaceClient.deleteRole({ workspaceId, roleId });
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
            ? t('workspace.roles.createTitle', 'New role')
            : t('workspace.roles.editTitle', 'Edit role')}
        </h2>
        <Card>
          <RoleEditor
            initialName={editing === 'new' ? '' : editing.name}
            initialPermissions={editing === 'new' ? 0n : editing.permissions}
            submitting={submitting}
            error={formError}
            canGrant={(bit) => can(myPermissions, bit)}
            onSubmit={submit}
            onCancel={() => setEditing(null)}
          />
        </Card>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="font-display text-lg text-ink-900">{t('workspace.roles.title', 'Roles')}</h2>
        {canManage && (
          <Button onPress={() => setEditing('new')}>{t('workspace.roles.new', 'New role')}</Button>
        )}
      </div>
      {actionError && <Banner tone="error">{actionError}</Banner>}
      {loading && (
        <p className="text-sm text-ink-500">{t('workspace.common.loading', 'Loading…')}</p>
      )}
      {error && <Banner tone="error">{error}</Banner>}
      <ul className="flex flex-col gap-2">
        {roles?.map((r) => (
          <li key={r.id}>
            <Card className="flex flex-wrap items-center justify-between gap-3 py-4">
              <div>
                <p className="font-medium text-ink-900">
                  {r.name}
                  {r.isDefault && (
                    <Pill tone="neutral" className="ml-2">
                      {t('workspace.roles.default', 'Default')}
                    </Pill>
                  )}
                </p>
                <p className="text-xs text-ink-500">
                  {t('workspace.roles.permCount', '{{count}} permissions', {
                    count: permissionCount(r.permissions),
                  })}
                </p>
              </div>
              {canManage && (
                <div className="flex gap-2">
                  <Button variant="ghost" onPress={() => setEditing(r)}>
                    {t('workspace.common.edit', 'Edit')}
                  </Button>
                  {!r.isDefault && (
                    <Button variant="danger" onPress={() => remove(r.id)}>
                      {t('workspace.common.delete', 'Delete')}
                    </Button>
                  )}
                </div>
              )}
            </Card>
          </li>
        ))}
      </ul>
    </div>
  );
}
