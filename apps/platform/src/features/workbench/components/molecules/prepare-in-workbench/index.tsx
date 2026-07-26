import { Code, ConnectError } from '@connectrpc/connect';
import { useNavigate } from '@tanstack/react-router';
import { Banner, Button, Card } from '@tendersbay/components/core';
import type { Bid } from '@tendersbay/proto/bid/v1/bid_pb';
import { usePostHog } from 'posthog-js/react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { deadlineInfo } from '~/features/account/components/organisms/tender-feed';
import { WorkbenchPicker } from '~/features/workbench/components/molecules/workbench-picker';
import { bidClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';
import { useWorkspaceStore } from '~/store/workspace';

function deadlineBucket(deadline: string): string {
  const dl = deadlineInfo(deadline, new Date());
  if (!dl) return 'none';
  if (dl.days < 0) return 'expired';
  if (dl.days <= 7) return 'lte_7';
  if (dl.days <= 14) return 'lte_14';
  if (dl.days <= 30) return 'lte_30';
  return 'gt_30';
}

/**
 * "Prepare in workbench" seam-closer — opens the WorkbenchPicker, tracks the
 * tender as a bid on selection, then navigates into its bid-detail page.
 * Route-supplied only from the authed tender-detail route (see
 * `routes/_authenticated/tenders/$id.tsx`); the public route never renders
 * this, so a logged-out visitor never sees the CTA.
 */
export function PrepareInWorkbench({ tenderId, deadline }: { tenderId: string; deadline: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const posthog = usePostHog();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const [open, setOpen] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  if (!isAuthenticated || !workspaceId) return null;

  async function add(workbenchId: string) {
    setBusyId(workbenchId);
    setError(null);
    try {
      const res = await bidClient.addBid({ workbenchId, tenderId });
      const bid: Bid | undefined = res.bid;
      posthog?.capture('bando_tracked', {
        location: 'tender_detail',
        source: 'tender_detail',
        fit_tier: bid?.fitTier || 'none',
        days_to_deadline_bucket: deadlineBucket(deadline),
      });
      await navigate({
        to: '/workspaces/$workspaceId/workbench/$workbenchId/bids/$bidId',
        params: { workspaceId: workspaceId as string, workbenchId, bidId: bid?.id ?? '' },
      });
    } catch (e: unknown) {
      const isDuplicate = e instanceof ConnectError && e.code === Code.AlreadyExists;
      setError(
        isDuplicate
          ? t('bid.errors.duplicate', 'This tender is already in that workbench.')
          : t('bid.errors.generic', 'Something went wrong. Please try again.'),
      );
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="flex flex-col items-end gap-2">
      <Button
        onPress={() => {
          posthog?.capture('tender_prepare_clicked', { location: 'tender_detail' });
          setOpen((v) => !v);
        }}
      >
        {t('bid.actions.prepare')}
      </Button>
      {open && (
        <Card className="w-full max-w-sm">
          <p className="mb-2 text-sm font-semibold text-ink-700">
            {t('bid.picker.title', 'Choose a workbench')}
          </p>
          {error && (
            <div className="mb-2">
              <Banner tone="error">{error}</Banner>
            </div>
          )}
          <WorkbenchPicker
            workspaceId={workspaceId}
            onSelect={(id) => void add(id)}
            busyId={busyId}
          />
        </Card>
      )}
    </div>
  );
}
