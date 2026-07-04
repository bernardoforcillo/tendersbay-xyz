import { createFileRoute } from '@tanstack/react-router';
import { WorkbenchLayout } from '~/features/workbench';

export const Route = createFileRoute(
  '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId',
)({
  component: WorkbenchLayout,
});
