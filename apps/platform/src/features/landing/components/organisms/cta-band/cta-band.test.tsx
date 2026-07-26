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

import { CtaBand } from './index';

describe('CtaBand', () => {
  it('renders the CTA heading and a button linking to the signup route', () => {
    renderWithI18n(<CtaBand />, 'en-ie');
    expect(
      screen.getByRole('heading', {
        name: 'Your agents are ready. The only thing missing is you.',
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Create your account' })).toHaveAttribute(
      'href',
      '/$locale/auth/signup',
    );
  });

  it('captures a cta_band click', () => {
    renderWithI18n(<CtaBand />, 'en-ie');
    fireEvent.click(screen.getByRole('link', { name: 'Create your account' }));
    expect(capture).toHaveBeenCalledWith('landing_cta_clicked', { location: 'cta_band' });
  });
});
