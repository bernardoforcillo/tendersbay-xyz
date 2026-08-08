import { render } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const captureMock = vi.fn();
const usePostHogMock = vi.fn(() => ({ capture: captureMock }));
vi.mock('posthog-js/react', () => ({ usePostHog: () => usePostHogMock() }));

import { useCaptureEvent } from './index';

function Probe({ onReady }: { onReady: (capture: ReturnType<typeof useCaptureEvent>) => void }) {
  onReady(useCaptureEvent());
  return null;
}

describe('useCaptureEvent', () => {
  beforeEach(() => {
    captureMock.mockClear();
    usePostHogMock.mockReturnValue({ capture: captureMock });
  });

  it('captures a catalogued event with a scrubbed payload', () => {
    let capture: ReturnType<typeof useCaptureEvent> | undefined;
    render(
      <Probe
        onReady={(c) => {
          capture = c;
        }}
      />,
    );
    capture?.('company_dossier_field_answered', {
      location: 'bid_detail',
      field_category: 'soa',
      prompted_by: 'tender',
    });
    expect(captureMock).toHaveBeenCalledWith('company_dossier_field_answered', {
      location: 'bid_detail',
      field_category: 'soa',
      prompted_by: 'tender',
    });
  });

  it('is a no-op when analytics is disabled', () => {
    usePostHogMock.mockReturnValue(null as never);
    let capture: ReturnType<typeof useCaptureEvent> | undefined;
    render(
      <Probe
        onReady={(c) => {
          capture = c;
        }}
      />,
    );
    expect(() =>
      capture?.('company_dossier_question_skipped', {
        location: 'bid_detail',
        field_category: 'turnover',
      }),
    ).not.toThrow();
    expect(captureMock).not.toHaveBeenCalled();
  });
});
