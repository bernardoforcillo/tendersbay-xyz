import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/workspaces/$workspaceId/invites')({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: '/workspaces/$workspaceId/settings/invites',
      params: { workspaceId: params.workspaceId },
    });
  },
});
