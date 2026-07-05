import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/roles')({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: '/workspaces/$workspaceId/settings/roles',
      params: { workspaceId: params.workspaceId },
    });
  },
});
