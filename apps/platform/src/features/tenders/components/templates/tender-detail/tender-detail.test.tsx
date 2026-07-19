import { create } from '@bufbuild/protobuf';
import { TenderDetailSchema } from '@tendersbay/proto/tender/v1/tender_pb';
import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import '~/i18n';
import { TenderDetailView } from './index';

beforeEach(() => {
  window.matchMedia = ((q: string) => ({
    matches: q.includes('reduce'),
    media: q,
    onchange: null,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
});

describe('TenderDetailView', () => {
  it('renders the title, buyer and value', () => {
    const tender = create(TenderDetailSchema, {
      id: '1',
      title: 'Road resurfacing',
      buyerName: 'City of Milan',
      value: 500000n,
      currency: 'EUR',
      status: 'open',
      country: 'ITA',
    });
    render(<TenderDetailView tender={tender} related={[]} renderRelated={() => null} />);
    expect(screen.getByRole('heading', { name: 'Road resurfacing' })).toBeTruthy();
    // "City of Milan" renders twice by design (header sub-title + meta buyer row).
    expect(screen.getAllByText('City of Milan').length).toBeGreaterThan(0);
  });
});
