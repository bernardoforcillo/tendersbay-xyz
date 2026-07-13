import { useNavigate } from '@tanstack/react-router';
import { Banner, Button, Card, Field, Select } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useWorkspaceContext } from '~/features/workspace/context';
import { useMembers } from '~/features/workspace/hooks';
import { can, Permission } from '~/features/workspace/permissions';
import { workspaceClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

export function WorkspaceSettingsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { workspaceId, workspace, myPermissions, refetch } = useWorkspaceContext();
  const userId = useAuthStore((s) => s.user?.id);
  const { data: members } = useMembers(workspaceId);
  const isOwner = workspace.ownerId === userId;
  const canManage = can(myPermissions, Permission.ManageWorkspace);

  const [name, setName] = useState(workspace.name);
  const [slug, setSlug] = useState(workspace.slug);
  const [newOwner, setNewOwner] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const otherMembers = (members ?? []).filter((m) => m.userId !== workspace.ownerId);

  async function saveDetails(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      await workspaceClient.updateWorkspace({ workspaceId, name: name.trim(), slug: slug.trim() });
      setSaved(true);
      refetch();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to save');
    } finally {
      setBusy(false);
    }
  }

  async function transfer() {
    if (!newOwner) return;
    setBusy(true);
    setError(null);
    try {
      await workspaceClient.transferOwnership({ workspaceId, newOwnerUserId: newOwner });
      refetch();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to transfer');
    } finally {
      setBusy(false);
    }
  }

  async function destroy() {
    setBusy(true);
    setError(null);
    try {
      await workspaceClient.deleteWorkspace({ workspaceId });
      await navigate({ to: '/workspaces' });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete');
      setBusy(false);
    }
  }

  async function leave() {
    setBusy(true);
    setError(null);
    try {
      await workspaceClient.leaveWorkspace({ workspaceId });
      await navigate({ to: '/workspaces' });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to leave');
      setBusy(false);
    }
  }

  return (
    <div className="flex max-w-2xl flex-col gap-8">
      {error && <Banner tone="error">{error}</Banner>}

      <section className="flex flex-col gap-4">
        <h2 className="font-display text-lg text-ink-900">
          {t('workspace.settings.detailsTitle', 'Workspace details')}
        </h2>
        <Card>
          <Form onSubmit={saveDetails} className="flex flex-col gap-4">
            <Field
              label={t('workspace.settings.name', 'Name')}
              value={name}
              onChange={setName}
              isRequired
              isDisabled={!canManage}
            />
            <Field
              label={t('workspace.settings.slug', 'Slug')}
              value={slug}
              onChange={setSlug}
              isDisabled={!canManage}
            />
            {canManage && (
              <div className="flex items-center gap-3">
                <Button type="submit" isDisabled={busy}>
                  {t('workspace.common.save', 'Save')}
                </Button>
                {saved && (
                  <span className="text-sm text-brand-700">
                    {t('workspace.common.saved', 'Saved')}
                  </span>
                )}
              </div>
            )}
          </Form>
        </Card>
      </section>

      {isOwner && (
        <section className="flex flex-col gap-4">
          <h2 className="font-display text-lg text-ink-900">
            {t('workspace.settings.transferTitle', 'Transfer ownership')}
          </h2>
          <Card className="flex flex-wrap items-end gap-3">
            <div className="flex-1">
              <Select
                label={t('workspace.settings.newOwner', 'New owner')}
                className="w-full"
                value={newOwner}
                onChange={(e) => setNewOwner(e.target.value)}
              >
                <option value="">{t('workspace.settings.selectMember', 'Select a member…')}</option>
                {otherMembers.map((m) => (
                  <option key={m.userId} value={m.userId}>
                    {m.user?.displayName || m.user?.email || m.userId}
                  </option>
                ))}
              </Select>
            </div>
            <Button isDisabled={busy || !newOwner} onPress={transfer}>
              {t('workspace.settings.transfer', 'Transfer')}
            </Button>
          </Card>
        </section>
      )}

      <section className="flex flex-col gap-4">
        <h2 className="font-display text-lg text-red-700">
          {t('workspace.settings.dangerTitle', 'Danger zone')}
        </h2>
        <Card className="flex flex-col gap-3">
          {isOwner ? (
            <div className="flex items-center justify-between gap-3">
              <p className="text-sm text-ink-600">
                {t(
                  'workspace.settings.deleteHint',
                  'Permanently delete this workspace and all its data.',
                )}
              </p>
              <Button variant="danger" isDisabled={busy} onPress={destroy}>
                {t('workspace.settings.delete', 'Delete workspace')}
              </Button>
            </div>
          ) : (
            <div className="flex items-center justify-between gap-3">
              <p className="text-sm text-ink-600">
                {t(
                  'workspace.settings.leaveHint',
                  'Leave this workspace. You can rejoin with a new invite.',
                )}
              </p>
              <Button variant="danger" isDisabled={busy} onPress={leave}>
                {t('workspace.settings.leave', 'Leave workspace')}
              </Button>
            </div>
          )}
        </Card>
      </section>
    </div>
  );
}
