import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceOverviewPage } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/')({
  component: WorkspaceOverviewPage,
});
