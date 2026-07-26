import { fireEvent, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

const { capture } = vi.hoisted(() => ({ capture: vi.fn() }));
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture }) }));
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: ({
    to,
    children,
    onClick,
    className,
  }: {
    to: string;
    children?: ReactNode;
    onClick?: () => void;
    className?: string;
  }) => (
    <a href={to} onClick={onClick} className={className}>
      {children}
    </a>
  ),
}));

import { SiteHeader } from './index';

describe('SiteHeader', () => {
  it('renders the logo, nav and language switcher in a banner', () => {
    renderWithI18n(<SiteHeader />, 'en-ie');
    expect(screen.getByRole('banner')).toBeInTheDocument();
    expect(screen.getByText('tenders')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'The agents' })).toBeInTheDocument();
  });

  it('captures a header cta click on the signup pill', () => {
    renderWithI18n(<SiteHeader />, 'en-ie');
    fireEvent.click(screen.getByRole('link', { name: 'Sign up' }));
    expect(capture).toHaveBeenCalledWith('landing_cta_clicked', { location: 'header' });
  });

  it('renders the signup pill as ghost while at the top of the page', () => {
    renderWithI18n(<SiteHeader />, 'en-ie');
    const pill = screen.getByRole('link', { name: 'Sign up' });
    expect(pill.className).toContain('border-brand-600');
    expect(pill.className).not.toContain('bg-brand-600');
  });
});
