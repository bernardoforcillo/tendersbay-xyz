import { render } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const captureMock = vi.fn();
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: captureMock }) }));

import type { AnalyticsEventProps } from '../../events';
import { useCaptureEventOnce } from './index';

function GapProbe({ evaluatedAt, gapCount }: { evaluatedAt: string | null; gapCount: number }) {
  const props: AnalyticsEventProps['eligibility_gap_shown'] = {
    location: 'scheda_gara',
    gap_count: gapCount,
    blocking: gapCount > 0,
    unknown_count: 0,
    coverage: 'full',
    reason: '',
  };
  useCaptureEventOnce('eligibility_gap_shown', evaluatedAt, props);
  return null;
}

describe('useCaptureEventOnce', () => {
  beforeEach(() => {
    captureMock.mockClear();
  });

  it('fires once for a snapshot and not again on re-render', () => {
    const { rerender } = render(<GapProbe evaluatedAt="2026-08-08T09:00:00Z" gapCount={2} />);
    rerender(<GapProbe evaluatedAt="2026-08-08T09:00:00Z" gapCount={2} />);
    rerender(<GapProbe evaluatedAt="2026-08-08T09:00:00Z" gapCount={2} />);
    expect(captureMock).toHaveBeenCalledTimes(1);
    expect(captureMock).toHaveBeenCalledWith('eligibility_gap_shown', {
      location: 'scheda_gara',
      gap_count: 2,
      blocking: true,
      unknown_count: 0,
      coverage: 'full',
      reason: '',
    });
  });

  it('fires again for a new snapshot, with the new snapshot values', () => {
    // The refetch after a captured answer: same component, new assessment, one
    // gap closed. That IS a new observation and must be counted as one.
    const { rerender } = render(<GapProbe evaluatedAt="2026-08-08T09:00:00Z" gapCount={2} />);
    rerender(<GapProbe evaluatedAt="2026-08-08T09:05:00Z" gapCount={1} />);
    expect(captureMock).toHaveBeenCalledTimes(2);
    expect(captureMock).toHaveBeenLastCalledWith(
      'eligibility_gap_shown',
      expect.objectContaining({ gap_count: 1, blocking: true }),
    );
  });

  it('stays silent while there is nothing real to report', () => {
    const { rerender } = render(<GapProbe evaluatedAt={null} gapCount={0} />);
    rerender(<GapProbe evaluatedAt={null} gapCount={0} />);
    expect(captureMock).not.toHaveBeenCalled();

    rerender(<GapProbe evaluatedAt="2026-08-08T09:00:00Z" gapCount={0} />);
    expect(captureMock).toHaveBeenCalledTimes(1);
  });
});
