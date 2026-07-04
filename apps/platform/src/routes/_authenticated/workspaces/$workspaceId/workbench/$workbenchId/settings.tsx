import { createFileRoute } from '@tanstack/react-router';
import { WorkbenchSettingsPage } from '~/features/workbench';

export const Route = createFileRoute(
  '/_authenticated/workspaces/$workspaceId/workbench/$workbenchId/settings',
)({
  component: WorkbenchSettingsPage,
});
