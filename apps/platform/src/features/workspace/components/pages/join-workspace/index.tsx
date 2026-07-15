import { useNavigate, useParams } from '@tanstack/react-router';
import { Banner, Button } from '@tendersbay/components/core';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AuthCard } from '~/features/auth/components/templates/auth-card';
import { useAsync } from '~/features/workspace/hooks';
import { detectLocale } from '~/i18n/detect-locale';
import { workspaceClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';

// Matches the kit Button's primary/md recipe — a real <a> (not a RAC Button)
// so the sign-in redirect stays a genuine, crawlable, middle-click-able link.
const SIGN_IN_LINK =
  'inline-flex h-10 items-center justify-center gap-2 rounded-xl bg-brand-600 px-4 text-sm font-semibold text-white outline-none transition-colors duration-150 hover:bg-brand-700 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 focus-visible:ring-offset-cream-100';

export function JoinWorkspacePage() {
  const { code } = useParams({ from: '/join/$code' });
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  // Locale-agnostic route: align i18n with the visitor's preferred locale.
  useEffect(() => {
    void i18n.changeLanguage(detectLocale());
  }, [i18n]);

  const preview = useCallback(() => workspaceClient.previewInviteLink({ code }), [code]);
  const { data, loading } = useAsync(preview);
  const [joining, setJoining] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function join() {
    setJoining(true);
    setError(null);
    try {
      const res = await workspaceClient.joinViaInviteLink({ code });
      if (res.workspace) {
        await navigate({
          to: '/workspaces/$workspaceId',
          params: { workspaceId: res.workspace.id },
        });
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to join workspace');
    } finally {
      setJoining(false);
    }
  }

  const loginHref = `/${detectLocale()}/auth/login?redirect=${encodeURIComponent(`/join/${code}`)}`;

  if (loading) {
    return <AuthCard heading={t('workspace.join.loading', 'Checking link…')} description="" />;
  }

  if (!data?.valid) {
    return (
      <AuthCard
        heading={t('workspace.join.invalidTitle', 'Link unavailable')}
        description={t(
          'workspace.join.invalidBody',
          'This invite link is invalid, expired, or fully used.',
        )}
      />
    );
  }

  return (
    <AuthCard
      heading={t('workspace.join.title', 'Join workspace')}
      description={t('workspace.join.body', 'Join {{workspace}} as {{role}}.', {
        workspace: data.workspaceName,
        role: data.roleName,
      })}
    >
      <div className="flex flex-col gap-4">
        {error && <Banner tone="error">{error}</Banner>}
        {isAuthenticated ? (
          <Button isDisabled={joining} onPress={join}>
            {joining
              ? t('workspace.join.joining', 'Joining…')
              : t('workspace.join.join', 'Join workspace')}
          </Button>
        ) : (
          <a href={loginHref} className={SIGN_IN_LINK}>
            {t('workspace.join.signIn', 'Sign in to join')}
          </a>
        )}
      </div>
    </AuthCard>
  );
}
