import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceSettingsPage } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/settings')({
  component: WorkspaceSettingsPage,
});
