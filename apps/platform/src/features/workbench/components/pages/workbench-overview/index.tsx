import { useNavigate } from '@tanstack/react-router';
import { Button, EmptyState } from '@tendersbay/components/core';
import { useTranslation } from 'react-i18next';
import { SearchDock } from '~/features/account/components/organisms';

export function WorkbenchOverviewPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <div className="flex flex-1 flex-col">
      <div className="flex flex-1 flex-col items-center justify-center px-4">
        <EmptyState
          title={t('workbench.overview.emptyTitle', 'Your workbench is ready')}
          description={t(
            'workbench.overview.emptyDescription',
            'Start by searching for tenders, creating a client profile, or inviting your team. Everything you do here is shared with workbench members.',
          )}
          action={
            <Button onPress={() => void navigate({ to: '/tenders' })}>
              {t('workbench.overview.emptyExplore', 'Open Explore')}
            </Button>
          }
        />
      </div>
      <div className="flex justify-center px-4 pb-6 pt-4">
        <SearchDock onPress={() => void navigate({ to: '/tenders' })} />
      </div>
    </div>
  );
}
