import { useCallback, useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useWorkbenchContext } from '~/features/workbench/context';
import { useAsync } from '~/features/workbench/hooks';
import { BTN_PRIMARY, ERROR_BOX } from '~/features/workbench/ui';
import { workbenchClient, workspaceClient } from '~/lib/api/client';

const SELECT =
  'rounded-lg border border-cream-300 bg-cream-50 px-2.5 py-1.5 text-sm text-ink-800 outline-none transition focus:border-brand-400 focus:ring-2 focus:ring-brand-100 disabled:opacity-50';

export function MemberAdd({
  roleId,
  existingUserIds,
  onAdded,
}: {
  roleId: string;
  existingUserIds: string[];
  onAdded: () => void;
}) {
  const { t } = useTranslation();
  const { workbench, workbenchId } = useWorkbenchContext();
  const fetchMembers = useCallback(
    () =>
      workspaceClient.listMembers({ workspaceId: workbench.workspaceId }).then((r) => r.members),
    [workbench.workspaceId],
  );
  const { data: wsMembers } = useAsync(fetchMembers);
  const [userId, setUserId] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const candidates = (wsMembers ?? []).filter((m) => !existingUserIds.includes(m.userId));

  async function add() {
    if (!userId) return;
    setBusy(true);
    setError(null);
    try {
      await workbenchClient.addWorkbenchMember({ workbenchId, userId, roleId });
      setUserId('');
      onAdded();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to add member');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-2">
      {error && (
        <p role="alert" className={ERROR_BOX}>
          {error}
        </p>
      )}
      <div className="flex items-center gap-2">
        <select
          aria-label={t('workbench.members.addLabel', 'Add member')}
          className={SELECT}
          value={userId}
          disabled={busy}
          onChange={(e) => setUserId(e.target.value)}
        >
          <option value="">{t('workbench.members.selectMember', 'Select a member…')}</option>
          {candidates.map((m) => (
            <option key={m.userId} value={m.userId}>
              {m.user?.displayName || m.user?.email || m.userId}
            </option>
          ))}
        </select>
        <Button className={BTN_PRIMARY} isDisabled={busy || !userId} onPress={add}>
          {t('workbench.members.add', 'Add')}
        </Button>
      </div>
    </div>
  );
}
