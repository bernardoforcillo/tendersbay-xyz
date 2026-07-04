import { createFileRoute } from '@tanstack/react-router';
import { WorkbenchRolesPage } from '~/features/workbench';

export const Route = createFileRoute(
  '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId/roles',
)({
  component: WorkbenchRolesPage,
});
