import { Banner } from '@tendersbay/components/core';
import { useTranslation } from 'react-i18next';
import { CompanyDossierForm } from '~/features/company/components/organisms';
import { useCompanyDossier } from '~/features/company/hooks';
import { useWorkspaceContext } from '~/features/workspace/context';

/**
 * Workspace settings page for the company dossier.
 *
 * The workspace IS the company, exactly as it is the client for the client
 * profile, so there is no company id and no company picker — one dossier per
 * workspace, sitting beside the client profile it complements. The client profile
 * says what this operator WANTS to bid on; the dossier says what it may lawfully
 * be admitted to.
 *
 * Nothing routes a blocked user here. A gap on a scheda gara is answered in the
 * gap row; this page is for deliberate correction.
 */
export function WorkspaceCompanyDossierPage() {
  const { t } = useTranslation();
  const { workspaceId } = useWorkspaceContext();
  const { data, loading, error, refetch } = useCompanyDossier(workspaceId);

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <div>
        <h2 className="font-display text-lg text-ink-900">
          {t('company.dossier.title', 'Company dossier')}
        </h2>
        <p className="mt-1 text-sm text-ink-500">
          {t(
            'company.dossier.description',
            'What we hold about your company. Used to check whether you may be admitted to a gara at all.',
          )}
        </p>
      </div>

      {error && <Banner tone="error">{error}</Banner>}

      {loading ? (
        <p className="text-sm text-ink-500">{t('workspace.common.loading', 'Loading…')}</p>
      ) : (
        <CompanyDossierForm
          workspaceId={workspaceId}
          dossier={data?.dossier ?? null}
          onChanged={refetch}
        />
      )}
    </div>
  );
}
