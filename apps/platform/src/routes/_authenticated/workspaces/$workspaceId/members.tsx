import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceMembersPage } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/members')({
  component: WorkspaceMembersPage,
});
