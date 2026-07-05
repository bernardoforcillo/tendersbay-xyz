import { createFileRoute } from '@tanstack/react-router';
import { AcceptInvitePage } from '~/features/workspace';

export const Route = createFileRoute('/$locale/workspace/accept-invite')({
  component: AcceptInvitePage,
});
