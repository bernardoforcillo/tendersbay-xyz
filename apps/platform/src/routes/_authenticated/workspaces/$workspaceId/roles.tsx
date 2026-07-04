import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceRolesPage } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/roles')({
  component: WorkspaceRolesPage,
});
