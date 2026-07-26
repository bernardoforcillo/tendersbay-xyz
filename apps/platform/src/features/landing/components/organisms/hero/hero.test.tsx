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
vi.mock('~/lib/api/client', () => ({
  tenderClient: { searchTenders: vi.fn().mockRejectedValue(new Error('no backend in tests')) },
}));

import { Hero } from './index';

describe('Hero', () => {
  it('renders the new headline and converts the primary CTA to signup', async () => {
    renderWithI18n(<Hero />, 'en-ie');
    await screen.findByRole('heading', { level: 1 });
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'Every public tender in 27 countries',
    );

    const primary = screen.getByRole('link', { name: /put your agents to work/i });
    expect(primary).toHaveAttribute('href', '/$locale/auth/signup');
    fireEvent.click(primary);
    expect(capture).toHaveBeenCalledWith('landing_cta_clicked', { location: 'hero' });

    expect(screen.getByRole('link', { name: /see how it works/i })).toHaveAttribute(
      'href',
      '#agents',
    );
    expect(screen.getByText('27 countries, one search')).toBeInTheDocument();
  });

  it('labels the sample deck honestly (no live/real-time claim)', async () => {
    renderWithI18n(<Hero />, 'en-ie');
    expect(await screen.findByText('Sample results')).toBeInTheDocument();
  });
});
