import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceCompanyDossierPage } from '~/features/company';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/settings/company')({
  component: WorkspaceCompanyDossierPage,
});
