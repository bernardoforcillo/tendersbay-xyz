import { useTranslation } from 'react-i18next';
import { useWorkspaceContext } from '~/features/workspace/context';
import { CARD } from '~/features/workspace/ui';

export function WorkspaceOverviewPage() {
  const { t } = useTranslation();
  const { workspace } = useWorkspaceContext();

  return (
    <div className="flex flex-col gap-4">
      <div className={CARD}>
        <h2 className="font-display text-lg text-ink-900">
          {t('workspace.overview.welcome', 'Welcome to')} {workspace.name}
        </h2>
        <p className="mt-2 text-sm text-ink-600">
          {t(
            'workspace.overview.workbenchSoon',
            'A collaborative workbench where your team can work on tenders together is coming soon. For now, manage members, roles and invitations from the tabs above.',
          )}
        </p>
      </div>
    </div>
  );
}
