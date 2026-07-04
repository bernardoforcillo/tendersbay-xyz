import { createFileRoute } from '@tanstack/react-router';
import { WorkbenchMembersPage } from '~/features/workbench';

export const Route = createFileRoute(
  '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId/members',
)({
  component: WorkbenchMembersPage,
});
