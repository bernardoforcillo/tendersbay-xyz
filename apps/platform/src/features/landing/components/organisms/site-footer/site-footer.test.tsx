import { screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

import { SiteFooter } from './index';

describe('SiteFooter', () => {
  it('renders the contact mailto link and copyright', () => {
    renderWithI18n(<SiteFooter />, 'en-ie');
    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /get in touch/i })).toHaveAttribute(
      'href',
      'mailto:me@bernardoforcillo.com',
    );
    expect(screen.getByText('© Bernardo Forcillo — All rights reserved')).toBeInTheDocument();
  });

  it('renders the link columns and social nav', () => {
    renderWithI18n(<SiteFooter />, 'en-ie');
    expect(screen.getByRole('navigation', { name: 'Product' })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Company' })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Resources' })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Follow tendersbay' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Vision' })).toHaveAttribute('href', '#vision');
  });
});
