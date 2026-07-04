import { useNavigate, useParams } from '@tanstack/react-router';
import { useQueryState } from 'nuqs';
import { useCallback, useState } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { useAsync } from '~/features/workspace/hooks';
import { BTN_PRIMARY, ERROR_BOX } from '~/features/workspace/ui';
import { workspaceClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

export function AcceptInvitePage() {
  const { locale } = useParams({ from: '/$locale/workspace/accept-invite' });
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const [token] = useQueryState('token');
  const preview = useCallback(
    () => workspaceClient.previewEmailInvite({ token: token ?? '' }),
    [token],
  );
  const { data, loading } = useAsync(preview);
  const [accepting, setAccepting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function accept() {
    if (!token) return;
    setAccepting(true);
    setError(null);
    try {
      const res = await workspaceClient.acceptEmailInvite({ token });
      if (res.workspace) {
        await navigate({
          to: '/workspaces/$workspaceId',
          params: { workspaceId: res.workspace.id },
        });
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to accept invitation');
    } finally {
      setAccepting(false);
    }
  }

  const returnTo = `/${locale}/workspace/accept-invite?token=${token ?? ''}`;
  const loginHref = `/${locale}/auth/login?redirect=${encodeURIComponent(returnTo)}`;

  if (loading) {
    return (
      <AuthCard heading={t('workspace.accept.loading', 'Checking invitation…')} description="" />
    );
  }

  if (!data?.valid) {
    return (
      <AuthCard
        heading={t('workspace.accept.invalidTitle', 'Invitation unavailable')}
        description={t(
          'workspace.accept.invalidBody',
          'This invitation is invalid or has expired.',
        )}
      />
    );
  }

  return (
    <AuthCard
      heading={t('workspace.accept.title', "You're invited")}
      description={t('workspace.accept.body', 'Join {{workspace}} as {{role}}.', {
        workspace: data.workspaceName,
        role: data.roleName,
      })}
    >
      <div className="flex flex-col gap-4">
        {error && (
          <p role="alert" className={ERROR_BOX}>
            {error}
          </p>
        )}
        {isAuthenticated ? (
          <Button className={BTN_PRIMARY} isDisabled={accepting} onPress={accept}>
            {accepting
              ? t('workspace.accept.accepting', 'Joining…')
              : t('workspace.accept.accept', 'Accept invitation')}
          </Button>
        ) : (
          <a href={loginHref} className={BTN_PRIMARY}>
            {t('workspace.accept.signIn', 'Sign in to accept')}
          </a>
        )}
      </div>
    </AuthCard>
  );
}
