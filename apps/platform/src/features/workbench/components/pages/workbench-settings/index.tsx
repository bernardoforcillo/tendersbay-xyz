import { useNavigate, useParams } from '@tanstack/react-router';
import { Banner, Button, Card, Field, Select } from '@tendersbay/components/core';
import { ArrowLeftRight, Eye, SquarePen, TriangleAlert } from 'lucide-react';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useWorkbenchMembers } from '~/features/workbench/hooks';
import { can, Permission } from '~/features/workbench/permissions';
import { workbenchClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

export function WorkbenchSettingsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { workspaceId } = useParams({
    from: '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId/settings',
  });
  const { workbenchId, workbench, myPermissions, refetch } = useWorkbenchContext();
  const userId = useAuthStore((s) => s.user?.id);
  const { data: members } = useWorkbenchMembers(workbenchId);
  const isOwner = workbench.ownerId === userId;
  const canManage = can(myPermissions, Permission.ManageWorkbench);

  const [name, setName] = useState(workbench.name);
  const [description, setDescription] = useState(workbench.description);
  const [visibility, setVisibility] = useState(workbench.visibility);
  const [newOwner, setNewOwner] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [confirming, setConfirming] = useState<'delete' | 'leave' | null>(null);

  const otherMembers = (members ?? []).filter((m) => m.userId !== workbench.ownerId);

  async function saveDetails(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      await workbenchClient.updateWorkbench({
        workbenchId,
        name: name.trim(),
        description: description.trim(),
      });
      setSaved(true);
      refetch();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to save');
    } finally {
      setBusy(false);
    }
  }

  async function changeVisibility(next: string) {
    setVisibility(next);
    setBusy(true);
    setError(null);
    try {
      await workbenchClient.changeVisibility({ workbenchId, visibility: next });
      refetch();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to change visibility');
    } finally {
      setBusy(false);
    }
  }

  async function transfer() {
    if (!newOwner) return;
    setBusy(true);
    setError(null);
    try {
      await workbenchClient.transferWorkbenchOwnership({ workbenchId, newOwnerUserId: newOwner });
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
      await workbenchClient.deleteWorkbench({ workbenchId });
      await navigate({ to: '/workspaces/$workspaceId/workbenches', params: { workspaceId } });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete');
      setBusy(false);
    }
  }

  async function leave() {
    setBusy(true);
    setError(null);
    try {
      await workbenchClient.leaveWorkbench({ workbenchId });
      await navigate({ to: '/workspaces/$workspaceId/workbenches', params: { workspaceId } });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to leave');
      setBusy(false);
    }
  }

  return (
    <div className="flex max-w-2xl flex-col gap-8">
      {error && <Banner tone="error">{error}</Banner>}

      <section className="flex flex-col gap-4">
        <h2 className="flex items-center gap-2 font-display text-lg text-ink-900">
          <SquarePen size={18} aria-hidden="true" className="text-ink-400" />
          {t('workbench.settings.detailsTitle', 'Workbench details')}
        </h2>
        <Card>
          <Form onSubmit={saveDetails} className="flex flex-col gap-4">
            <Field
              label={t('workbench.settings.name', 'Name')}
              value={name}
              onChange={setName}
              isRequired
              isDisabled={!canManage}
            />
            <Field
              label={t('workbench.settings.description', 'Description')}
              value={description}
              onChange={setDescription}
              isDisabled={!canManage}
            />
            {canManage && (
              <div className="flex items-center gap-3">
                <Button type="submit" isDisabled={busy}>
                  {t('workbench.common.save', 'Save')}
                </Button>
                {saved && (
                  <span className="text-sm text-brand-700">
                    {t('workbench.common.saved', 'Saved')}
                  </span>
                )}
              </div>
            )}
          </Form>
        </Card>
      </section>

      <section className="flex flex-col gap-4">
        <h2 className="flex items-center gap-2 font-display text-lg text-ink-900">
          <Eye size={18} aria-hidden="true" className="text-ink-400" />
          {t('workbench.settings.visibilityTitle', 'Visibility')}
        </h2>
        <Card className="flex flex-wrap items-end gap-3">
          <div className="flex-1">
            <Select
              label={t('workbench.settings.visibility', 'Visibility')}
              className="w-full"
              value={visibility}
              disabled={!canManage || busy}
              onChange={(e) => changeVisibility(e.target.value)}
            >
              <option value="private">{t('workbench.visibility.private', 'Private')}</option>
              <option value="shared">{t('workbench.visibility.shared', 'Shared')}</option>
            </Select>
          </div>
        </Card>
      </section>

      {isOwner && (
        <section className="flex flex-col gap-4">
          <h2 className="flex items-center gap-2 font-display text-lg text-ink-900">
            <ArrowLeftRight size={18} aria-hidden="true" className="text-ink-400" />
            {t('workbench.settings.transferTitle', 'Transfer ownership')}
          </h2>
          <Card className="flex flex-wrap items-end gap-3">
            <div className="flex-1">
              <Select
                label={t('workbench.settings.newOwner', 'New owner')}
                className="w-full"
                value={newOwner}
                onChange={(e) => setNewOwner(e.target.value)}
              >
                <option value="">{t('workbench.settings.selectMember', 'Select a member…')}</option>
                {otherMembers.map((m) => (
                  <option key={m.userId} value={m.userId}>
                    {m.user?.displayName || m.user?.email || m.userId}
                  </option>
                ))}
              </Select>
            </div>
            <Button isDisabled={busy || !newOwner} onPress={transfer}>
              {t('workbench.settings.transfer', 'Transfer')}
            </Button>
          </Card>
        </section>
      )}

      <section className="flex flex-col gap-4">
        <h2 className="flex items-center gap-2 font-display text-lg text-red-700">
          <TriangleAlert size={18} aria-hidden="true" />
          {t('workbench.settings.dangerTitle', 'Danger zone')}
        </h2>
        <Card className="flex flex-col gap-3">
          {isOwner ? (
            <div className="flex flex-wrap items-center justify-between gap-3">
              <p className="text-sm text-ink-600">
                {t(
                  'workbench.settings.deleteHint',
                  'Permanently delete this workbench and all its data.',
                )}
              </p>
              {confirming === 'delete' ? (
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-red-700">
                    {t('workbench.settings.confirmDelete', 'Delete this workbench?')}
                  </span>
                  <Button variant="danger" isDisabled={busy} onPress={destroy}>
                    {t('workbench.common.confirm', 'Confirm')}
                  </Button>
                  <Button variant="ghost" isDisabled={busy} onPress={() => setConfirming(null)}>
                    {t('workbench.common.cancel', 'Cancel')}
                  </Button>
                </div>
              ) : (
                <Button variant="danger" isDisabled={busy} onPress={() => setConfirming('delete')}>
                  {t('workbench.settings.delete', 'Delete workbench')}
                </Button>
              )}
            </div>
          ) : (
            <div className="flex flex-wrap items-center justify-between gap-3">
              <p className="text-sm text-ink-600">
                {t(
                  'workbench.settings.leaveHint',
                  'Leave this workbench. You can be re-added by a manager.',
                )}
              </p>
              {confirming === 'leave' ? (
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-red-700">
                    {t('workbench.settings.confirmLeave', 'Leave this workbench?')}
                  </span>
                  <Button variant="danger" isDisabled={busy} onPress={leave}>
                    {t('workbench.common.confirm', 'Confirm')}
                  </Button>
                  <Button variant="ghost" isDisabled={busy} onPress={() => setConfirming(null)}>
                    {t('workbench.common.cancel', 'Cancel')}
                  </Button>
                </div>
              ) : (
                <Button variant="danger" isDisabled={busy} onPress={() => setConfirming('leave')}>
                  {t('workbench.settings.leave', 'Leave workbench')}
                </Button>
              )}
            </div>
          )}
        </Card>
      </section>
    </div>
  );
}
