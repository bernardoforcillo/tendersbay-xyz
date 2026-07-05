import { createFileRoute } from '@tanstack/react-router';
import { WorkbenchesListPage } from '~/features/workbench';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/workbenches')({
  component: WorkbenchesListPage,
});
