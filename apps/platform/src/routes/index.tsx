import { createFileRoute, redirect } from '@tanstack/react-router';
import { homeTarget } from '~/features/workspace/home';
import { detectLocale } from '~/i18n/detect-locale';
import { useWorkspaceStore } from '~/store/workspace';

export const Route = createFileRoute('/')({
  beforeLoad: ({ context }) => {
    if (!context.auth.isAuthenticated) {
      throw redirect({ to: '/$locale', params: { locale: detectLocale() } });
    }
    const target = homeTarget(useWorkspaceStore.getState().currentWorkspaceId);
    if (target.kind === 'workspace') {
      throw redirect({
        to: '/workspaces/$workspaceId',
        params: { workspaceId: target.workspaceId },
      });
    }
    throw redirect({ to: '/workspaces' });
  },
  component: () => null,
});
