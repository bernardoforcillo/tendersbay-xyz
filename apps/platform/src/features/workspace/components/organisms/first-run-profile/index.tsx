import { Button, Card } from '@tendersbay/components/core';
import type { ClientProfile } from '@tendersbay/proto/workspace/v1/workspace_pb';
import { type ReactNode, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ClientProfileForm } from '~/features/account/components/organisms';
import { workspaceClient } from '~/lib/api/client';
import { readAndClearLandingCarryOver } from '~/lib/landing-carry-over';
import { useFirstRunStore } from '~/store/first-run';

export type FirstRunProfileProps = {
  workspaceId: string;
  /** The page's normal content — rendered whenever first-run capture isn't shown. */
  children: ReactNode;
};

type Status = 'checking' | 'needed' | 'not-needed';

function carryOverInitial(workspaceId: string): ClientProfile | undefined {
  const carryOver = readAndClearLandingCarryOver();
  if (!carryOver) return undefined;
  return {
    $typeName: 'workspace.v1.ClientProfile',
    workspaceId,
    sectors: [],
    countries: [],
    regions: [],
    procedureTypes: [],
    valueMin: 0n,
    valueMinSet: false,
    valueMax: 0n,
    valueMaxSet: false,
    notes: carryOver.query,
    updatedAt: '',
  };
}

/**
 * Self-contained first-run gate for a freshly created workspace: checks
 * `GetClientProfile` directly (its own lightweight signal, deliberately not
 * `useClientShortlist`'s `needsProfile` — that hook drives a *separate*
 * Explore/Today nudge and must stay independent so the two don't race or
 * double-prompt) and, when no profile exists yet and the advisor hasn't
 * skipped it this session, shows `ClientProfileForm` instead of the page's
 * normal content. Skipping records the workspace in `useFirstRunStore`
 * (sessionStorage) — it never writes an empty profile to the server. A
 * pre-seeded landing-search carry-over (read once and cleared) pre-fills
 * Notes so a visitor's pre-signup search isn't lost.
 */
export function FirstRunProfile({ workspaceId, children }: FirstRunProfileProps) {
  const { t } = useTranslation();
  const skipped = useFirstRunStore((s) => s.hasSkipped(workspaceId));
  const skipWorkspace = useFirstRunStore((s) => s.skipWorkspace);
  const [status, setStatus] = useState<Status>(skipped ? 'not-needed' : 'checking');
  const [initial, setInitial] = useState<ClientProfile | undefined>(undefined);

  useEffect(() => {
    if (skipped) {
      setStatus('not-needed');
      return;
    }
    let active = true;
    setStatus('checking');
    workspaceClient
      .getClientProfile({ workspaceId })
      .then((res) => {
        if (!active) return;
        if (res.exists) {
          setStatus('not-needed');
        } else {
          setInitial(carryOverInitial(workspaceId));
          setStatus('needed');
        }
      })
      .catch(() => {
        // Fail open — a network hiccup on this lightweight check must never
        // block the advisor from the page itself.
        if (active) setStatus('not-needed');
      });
    return () => {
      active = false;
    };
  }, [workspaceId, skipped]);

  if (status !== 'needed') {
    return <>{children}</>;
  }

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-6 px-4 py-8 lg:px-8">
      <div>
        <h1 className="font-display text-2xl text-ink-900">
          {t('workspace.firstRun.title', 'Add a client bid profile')}
        </h1>
        <p className="mt-1 text-sm text-ink-500">
          {t(
            'workspace.firstRun.subtitle',
            'Tell us what this client bids on so recommendations start sharp. You can finish this later from Settings.',
          )}
        </p>
      </div>
      <Card>
        <ClientProfileForm
          workspaceId={workspaceId}
          initial={initial}
          location="first_run"
          onSaved={() => setStatus('not-needed')}
        />
      </Card>
      <Button
        variant="quiet"
        className="self-start"
        onPress={() => {
          skipWorkspace(workspaceId);
          setStatus('not-needed');
        }}
      >
        {t('workspace.firstRun.skip', 'Skip for now')}
      </Button>
    </div>
  );
}
