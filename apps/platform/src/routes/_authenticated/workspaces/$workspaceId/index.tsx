import { createFileRoute } from '@tanstack/react-router';
import { WorkspaceTodayPage } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/')({
  component: WorkspaceTodayPage,
});
