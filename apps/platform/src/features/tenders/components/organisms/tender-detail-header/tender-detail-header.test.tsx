import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (k: string, o?: unknown) =>
      typeof o === 'string' ? o : ((o as { defaultValue?: string })?.defaultValue ?? k),
    i18n: { language: 'en-ie' },
  }),
}));
vi.mock('~/features/account/components/organisms/tender-feed', () => ({
  countryFlag: () => null,
  countryName: () => null,
  deadlineInfo: () => null,
}));

import { TenderDetailHeader } from './index';

const tender = {
  title: 'Road works',
  buyerName: 'City',
  country: '',
  source: '',
  deadline: '',
  // biome-ignore lint/suspicious/noExplicitAny: minimal tender for the header
} as any;

describe('TenderDetailHeader actions slot', () => {
  it('renders the actions node when provided', () => {
    render(<TenderDetailHeader tender={tender} actions={<button type="button">Prepare</button>} />);
    expect(screen.getByText('Prepare')).toBeTruthy();
  });

  it('renders no actions region when omitted', () => {
    render(<TenderDetailHeader tender={tender} />);
    expect(screen.queryByText('Prepare')).toBeNull();
  });
});
