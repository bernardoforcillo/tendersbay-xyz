import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceLayout } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId')({
  component: WorkspaceLayout,
});
