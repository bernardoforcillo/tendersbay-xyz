import { renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const capture = vi.fn();
vi.mock('~/analytics', async () => {
  const actual = await vi.importActual<typeof import('~/analytics')>('~/analytics');
  return { ...actual, useCaptureEvent: () => capture };
});

const { useReminderAttribution } = await import('./index');

function at(search: string) {
  window.history.replaceState(null, '', `/workspaces/w/workbench/wb/bids/b1${search}`);
}

afterEach(() => {
  capture.mockClear();
  at('');
});

describe('useReminderAttribution', () => {
  it('captures the click and the bucket that produced it', () => {
    at('?src=reminder&b=7');
    renderHook(() => useReminderAttribution('b1'));
    expect(capture).toHaveBeenCalledWith('reminder_link_opened', {
      location: 'bid_detail',
      bucket: '7',
    });
  });

  // A reload or a URL pasted to a colleague must not each count as another
  // click on a reminder that was opened once.
  it('strips the markers so the visit cannot be attributed twice', () => {
    at('?src=reminder&b=7');
    renderHook(() => useReminderAttribution('b1'));
    expect(window.location.search).toBe('');

    renderHook(() => useReminderAttribution('b1'));
    expect(capture).toHaveBeenCalledTimes(1);
  });

  it('keeps unrelated query params', () => {
    at('?src=reminder&b=3&lot=2');
    renderHook(() => useReminderAttribution('b1'));
    expect(window.location.search).toBe('?lot=2');
  });

  // The bucket arrives from a URL anyone can edit. An unrecognised value must
  // become '' rather than opening a fifth cohort — but the event still fires,
  // because a link that lost its bucket is still a reminder that worked.
  it('still fires with an empty bucket when the marker is unrecognised', () => {
    at('?src=reminder&b=99');
    renderHook(() => useReminderAttribution('b1'));
    expect(capture).toHaveBeenCalledWith('reminder_link_opened', {
      location: 'bid_detail',
      bucket: '',
    });
  });

  it('does nothing for an ordinary visit', () => {
    at('');
    renderHook(() => useReminderAttribution('b1'));
    expect(capture).not.toHaveBeenCalled();
  });

  it('waits for the bid id rather than attributing a visit it cannot name', () => {
    at('?src=reminder&b=7');
    renderHook(() => useReminderAttribution(undefined));
    expect(capture).not.toHaveBeenCalled();
    expect(window.location.search).toBe('?src=reminder&b=7');
  });
});
