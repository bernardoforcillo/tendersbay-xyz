import { createFileRoute } from '@tanstack/react-router';
import { JoinWorkspacePage } from '~/features/workspace';

export const Route = createFileRoute('/join/$code')({
  component: JoinWorkspacePage,
});
