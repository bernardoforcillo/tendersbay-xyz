import { useNavigate, useParams } from '@tanstack/react-router';
import { Banner, Button } from '@tendersbay/components/core';
import { useQueryState } from 'nuqs';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { useAsync } from '~/features/workspace/hooks';
import { workspaceClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

// Matches the kit Button's primary/md recipe — a real <a> (not a RAC Button)
// so the sign-in redirect stays a genuine, crawlable, middle-click-able link.
const SIGN_IN_LINK =
  'inline-flex h-10 items-center justify-center gap-2 rounded-xl bg-brand-600 px-4 text-sm font-semibold text-white outline-none transition-colors duration-150 hover:bg-brand-700 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 focus-visible:ring-offset-cream-100';

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
        {error && <Banner tone="error">{error}</Banner>}
        {isAuthenticated ? (
          <Button isDisabled={accepting} onPress={accept}>
            {accepting
              ? t('workspace.accept.accepting', 'Joining…')
              : t('workspace.accept.accept', 'Accept invitation')}
          </Button>
        ) : (
          <a href={loginHref} className={SIGN_IN_LINK}>
            {t('workspace.accept.signIn', 'Sign in to accept')}
          </a>
        )}
      </div>
    </AuthCard>
  );
}
