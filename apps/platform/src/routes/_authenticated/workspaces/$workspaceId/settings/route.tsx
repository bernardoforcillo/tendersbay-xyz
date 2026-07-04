import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceSettingsLayout } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/settings')({
  component: WorkspaceSettingsLayout,
});
