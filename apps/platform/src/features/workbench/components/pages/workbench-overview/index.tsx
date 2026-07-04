import { useTranslation } from 'react-i18next';

export function WorkbenchOverviewPage() {
  const { t } = useTranslation();
  // The workbench name/description live in the layout header; the overview root
  // is an empty surface until item types land in a later spec.
  return (
    <p className="text-sm text-ink-500">
      {t('workbench.overview.empty', 'This workbench is empty for now.')}
    </p>
  );
}
