import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceClientProfilePage } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/settings/profile')({
  component: WorkspaceClientProfilePage,
});
