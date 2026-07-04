import { useNavigate, useParams } from '@tanstack/react-router';
import { useState } from 'react';
import { Button, Form, Input, Label, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useWorkbenchMembers } from '~/features/workbench/hooks';
import { can, Permission } from '~/features/workbench/permissions';
import { BTN_DANGER, BTN_PRIMARY, CARD, ERROR_BOX, INPUT, LABEL } from '~/features/workbench/ui';
import { workbenchClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

const SELECT =
  'rounded-lg border border-cream-300 bg-cream-50 px-2.5 py-2 text-sm text-ink-800 outline-none transition focus:border-brand-400 focus:ring-2 focus:ring-brand-100';

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
      {error && (
        <p role="alert" className={ERROR_BOX}>
          {error}
        </p>
      )}

      <section className="flex flex-col gap-4">
        <h2 className="font-display text-lg text-ink-900">
          {t('workbench.settings.detailsTitle', 'Workbench details')}
        </h2>
        <div className={CARD}>
          <Form onSubmit={saveDetails} className="flex flex-col gap-4">
            <TextField
              value={name}
              onChange={setName}
              isRequired
              isDisabled={!canManage}
              className="flex flex-col gap-1.5"
            >
              <Label className={LABEL}>{t('workbench.settings.name', 'Name')}</Label>
              <Input className={INPUT} />
            </TextField>
            <TextField
              value={description}
              onChange={setDescription}
              isDisabled={!canManage}
              className="flex flex-col gap-1.5"
            >
              <Label className={LABEL}>{t('workbench.settings.description', 'Description')}</Label>
              <Input className={INPUT} />
            </TextField>
            {canManage && (
              <div className="flex items-center gap-3">
                <Button type="submit" isDisabled={busy} className={BTN_PRIMARY}>
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
        </div>
      </section>

      <section className="flex flex-col gap-4">
        <h2 className="font-display text-lg text-ink-900">
          {t('workbench.settings.visibilityTitle', 'Visibility')}
        </h2>
        <div className={`${CARD} flex flex-wrap items-end gap-3`}>
          <label className="flex flex-1 flex-col gap-1.5">
            <span className={LABEL}>{t('workbench.settings.visibility', 'Visibility')}</span>
            <select
              className={SELECT}
              value={visibility}
              disabled={!canManage || busy}
              onChange={(e) => changeVisibility(e.target.value)}
            >
              <option value="private">{t('workbench.visibility.private', 'Private')}</option>
              <option value="shared">{t('workbench.visibility.shared', 'Shared')}</option>
            </select>
          </label>
        </div>
      </section>

      {isOwner && (
        <section className="flex flex-col gap-4">
          <h2 className="font-display text-lg text-ink-900">
            {t('workbench.settings.transferTitle', 'Transfer ownership')}
          </h2>
          <div className={`${CARD} flex flex-wrap items-end gap-3`}>
            <label className="flex flex-1 flex-col gap-1.5">
              <span className={LABEL}>{t('workbench.settings.newOwner', 'New owner')}</span>
              <select
                className={SELECT}
                value={newOwner}
                onChange={(e) => setNewOwner(e.target.value)}
              >
                <option value="">{t('workbench.settings.selectMember', 'Select a member…')}</option>
                {otherMembers.map((m) => (
                  <option key={m.userId} value={m.userId}>
                    {m.user?.displayName || m.user?.email || m.userId}
                  </option>
                ))}
              </select>
            </label>
            <Button isDisabled={busy || !newOwner} className={BTN_PRIMARY} onPress={transfer}>
              {t('workbench.settings.transfer', 'Transfer')}
            </Button>
          </div>
        </section>
      )}

      <section className="flex flex-col gap-4">
        <h2 className="font-display text-lg text-red-700">
          {t('workbench.settings.dangerTitle', 'Danger zone')}
        </h2>
        <div className={`${CARD} flex flex-col gap-3`}>
          {isOwner ? (
            <div className="flex items-center justify-between gap-3">
              <p className="text-sm text-ink-600">
                {t(
                  'workbench.settings.deleteHint',
                  'Permanently delete this workbench and all its data.',
                )}
              </p>
              <Button isDisabled={busy} className={BTN_DANGER} onPress={destroy}>
                {t('workbench.settings.delete', 'Delete workbench')}
              </Button>
            </div>
          ) : (
            <div className="flex items-center justify-between gap-3">
              <p className="text-sm text-ink-600">
                {t(
                  'workbench.settings.leaveHint',
                  'Leave this workbench. You can be re-added by a manager.',
                )}
              </p>
              <Button isDisabled={busy} className={BTN_DANGER} onPress={leave}>
                {t('workbench.settings.leave', 'Leave workbench')}
              </Button>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
