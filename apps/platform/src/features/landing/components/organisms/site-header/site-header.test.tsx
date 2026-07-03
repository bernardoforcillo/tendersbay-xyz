import { screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

import { SiteHeader } from './index';

describe('SiteHeader', () => {
  it('renders the logo, nav and language switcher in a banner', () => {
    renderWithI18n(<SiteHeader />, 'en-ie');
    expect(screen.getByRole('banner')).toBeInTheDocument();
    expect(screen.getByText('tenders')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'The agents' })).toBeInTheDocument();
  });
});
