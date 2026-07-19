import { Banner, Card } from '@tendersbay/components/core';
import type { ClientProfile } from '@tendersbay/proto/workspace/v1/workspace_pb';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ClientProfileForm } from '~/features/account/components/organisms';
import { useWorkspaceContext } from '~/features/workspace/context';
import { workspaceClient } from '~/lib/api/client';

/**
 * Workspace-scoped settings page for the client bid profile — the same
 * `ClientProfileForm` used by first-run capture (Task A-firstrun) and, later,
 * Explore (Task 15), pre-filled from `GetClientProfile` when one already
 * exists. One profile per workspace/client, so this lives under workspace
 * settings rather than account settings.
 */
export function WorkspaceClientProfilePage() {
  const { t } = useTranslation();
  const { workspaceId } = useWorkspaceContext();
  const [profile, setProfile] = useState<ClientProfile | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    workspaceClient
      .getClientProfile({ workspaceId })
      .then((res) => {
        if (!active) return;
        setProfile(res.exists ? res.profile : undefined);
      })
      .catch((e: unknown) => {
        if (!active) return;
        setError(e instanceof Error ? e.message : 'Failed to load the client profile');
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [workspaceId]);

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <div>
        <h2 className="font-display text-lg text-ink-900">
          {t('workspace.clientProfile.title', 'Client bid profile')}
        </h2>
        <p className="mt-1 text-sm text-ink-500">
          {t(
            'workspace.clientProfile.subtitle',
            'What this client bids on — sectors, countries, and value range — drives every recommendation.',
          )}
        </p>
      </div>

      {error && <Banner tone="error">{error}</Banner>}
      {saved && (
        <Banner tone="success">{t('workspace.clientProfile.saved', 'Profile saved.')}</Banner>
      )}

      {loading ? (
        <p className="text-sm text-ink-500">{t('workspace.common.loading', 'Loading…')}</p>
      ) : (
        <Card>
          <ClientProfileForm
            workspaceId={workspaceId}
            initial={profile}
            location="settings"
            onSaved={(next) => {
              setProfile(next);
              setSaved(true);
            }}
          />
        </Card>
      )}
    </div>
  );
}
