import { createFileRoute } from '@tanstack/react-router';
import { WorkspacesListPage } from '~/features/workspace';

export const Route = createFileRoute('/_authenticated/workspaces/')({
  component: WorkspacesListPage,
});
