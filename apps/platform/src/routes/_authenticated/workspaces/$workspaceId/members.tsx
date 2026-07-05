import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/members')({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: '/workspaces/$workspaceId/settings/members',
      params: { workspaceId: params.workspaceId },
    });
  },
});
