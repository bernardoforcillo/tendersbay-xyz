import { useTranslation } from 'react-i18next';
import { useWorkbenchContext } from '~/features/workbench/context';
import { CARD } from '~/features/workbench/ui';

export function WorkbenchOverviewPage() {
  const { t } = useTranslation();
  const { workbench } = useWorkbenchContext();
  return (
    <div className={CARD}>
      <h2 className="font-display text-lg text-ink-900">{workbench.name}</h2>
      <p className="mt-1 text-sm text-ink-500">
        {workbench.description || t('workbench.overview.noDescription', 'No description yet.')}
      </p>
      <span className="mt-3 inline-block rounded-full bg-cream-200 px-2 py-0.5 text-xs text-ink-600">
        {t(`workbench.visibility.${workbench.visibility}`, workbench.visibility)}
      </span>
    </div>
  );
}
