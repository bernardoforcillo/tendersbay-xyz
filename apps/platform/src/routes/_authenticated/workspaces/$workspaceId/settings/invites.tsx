import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceInvitesPage } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/settings/invites')({
  component: WorkspaceInvitesPage,
});
